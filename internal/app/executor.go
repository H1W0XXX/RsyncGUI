package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gokrazy/rsync/rsyncclient"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type JobManager struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	hosts *HostRegistry
}

func NewJobManager(hosts *HostRegistry) *JobManager {
	return &JobManager{
		jobs:  make(map[string]*Job),
		hosts: hosts,
	}
}

func (m *JobManager) NewJob(req TransferRequest, plan *TransferPlan) *Job {
	id := uuid.New().String()
	job := &Job{
		ID:        id,
		Request:   req,
		Plan:      *plan,
		Status:    JobPending,
		CreatedAt: time.Now(),
		LogLines:  make([]string, 0, 128),
	}
	m.mu.Lock()
	m.jobs[id] = job
	m.mu.Unlock()
	return job
}

func (m *JobManager) GetJob(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

func (m *JobManager) ListJobs() []*Job {
	m.mu.RLock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	m.mu.RUnlock()

	// 最新的排前面；如果 CreatedAt 相同，用 ID 稳定一下
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (m *JobManager) StartJob(job *Job) { go m.runJob(job) }

func (m *JobManager) runJob(job *Job) {
	job.mu.Lock()
	job.Status = JobRunning
	job.StartedAt = time.Now()
	job.mu.Unlock()

	ctx := context.Background()

	runner, err := m.buildRunner(job)
	if err != nil {
		job.mu.Lock()
		job.Status = JobFailed
		job.LogLines = append(job.LogLines, "build runner failed: "+err.Error())
		job.EndedAt = time.Now()
		job.mu.Unlock()
		return
	}

	err = runner(ctx)

	job.mu.Lock()
	defer job.mu.Unlock()
	if err != nil {
		job.Status = JobFailed
		job.LogLines = append(job.LogLines, "transfer failed: "+err.Error())
	} else {
		job.Status = JobOK
	}
	job.EndedAt = time.Now()
}

type DialTarget struct {
	Host string
	Port int
}

func decideUseLan(req TransferRequest) bool {
	return strings.EqualFold(req.Options.Profile, "LAN")
}

func dialFor(c *HostConfig, useLan bool) DialTarget {
	if useLan && c.LanHost != "" && c.LanPort > 0 {
		return DialTarget{Host: c.LanHost, Port: c.LanPort}
	}
	port := c.Port
	if port == 0 {
		port = 22
	}
	return DialTarget{Host: c.Host, Port: port}
}

func (m *JobManager) getHost(name string) (*Host, error) {
	h, ok := m.hosts.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown host %q", name)
	}
	return h, nil
}

func (m *JobManager) buildRunner(job *Job) (func(context.Context) error, error) {
	plan := job.Plan
	//req := job.Request

	srcHost := plan.Source.HostName
	dstHost := plan.Dest.HostName

	// 本机↔远程：纯 Go（Windows 也能跑，不依赖本机 rsync/ssh）
	if plan.Mode == ExecLocal {
		switch {
		case srcHost == "local" && dstHost == "local":
			return func(ctx context.Context) error {
				return m.runLocalLocal(job, ctx)
			}, nil
		case srcHost == "local" && dstHost != "local":
			return func(ctx context.Context) error {
				return m.runLocalToRemote_GoRsync(job, ctx)
			}, nil
		case srcHost != "local" && dstHost == "local":
			return func(ctx context.Context) error {
				return m.runRemoteToLocal_GoRsync(job, ctx)
			}, nil
		default:
			return nil, fmt.Errorf("ExecLocal unexpected: %s -> %s", srcHost, dstHost)
		}
	}

	// 远程↔远程：在某台远程上跑命令行 rsync（第一跳用 Go SSH，带“内置 agent 转发”免 ssh-add）
	if plan.Mode == ExecOnSource || plan.Mode == ExecOnDest {
		return func(ctx context.Context) error {
			return m.runRemoteToRemote_OneHopSSH(job, ctx)
		}, nil
	}

	return nil, fmt.Errorf("unsupported plan mode: %s", plan.Mode)
}

type jobLineWriter struct {
	job *Job
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *jobLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, _ := w.buf.Write(p)

	for {
		b := w.buf.Bytes()
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(b[:i]), "\r")
		w.buf.Next(i + 1)
		w.job.mu.Lock()
		w.job.LogLines = append(w.job.LogLines, line)
		w.job.mu.Unlock()
	}
	return n, nil
}

func (w *jobLineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return
	}
	line := strings.TrimRight(w.buf.String(), "\r\n")
	w.buf.Reset()
	w.job.mu.Lock()
	w.job.LogLines = append(w.job.LogLines, line)
	w.job.mu.Unlock()

}

func buildRsyncArgs(opts *RsyncOptions) []string {
	var args []string

	if opts.Archive {
		args = append(args, "-a")
	} else {
		args = append(args, "-r")
	}
	if opts.Compress {
		args = append(args, "-z")
	}
	if opts.Delete {
		args = append(args, "--delete")
	}
	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	if opts.BwLimit > 0 {
		args = append(args, fmt.Sprintf("--bwlimit=%d", opts.BwLimit))
	}
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}

	return args
}

// ============================
// 1) local <-> local（不依赖外部 rsync：用 rsynccmd 最简单，但你现在没引它也行）
// 这里先用系统 rsync（你如果要彻底去掉外部依赖，我再给你换 rsynccmd 版）
// ============================
func (m *JobManager) runLocalLocal(job *Job, ctx context.Context) error {
	src := job.Plan.Source.Path
	dst := job.Plan.Dest.Path

	args := buildRsyncArgs(&job.Request.Options)
	args = append(args, src, dst)

	w := &jobLineWriter{job: job}
	job.mu.Lock()
	job.LogLines = append(job.LogLines, "rsync "+strings.Join(args, " "))
	job.mu.Unlock()

	cmd := exec.CommandContext(ctx, "rsync", args...)
	cmd.Stdout = w
	cmd.Stderr = w
	err := cmd.Run()
	w.Flush()
	return err
}

// ============================
// 2) local -> remote：Go 内置 SSH + gokrazy/rsyncclient（本机不需要 rsync/ssh）
// ============================
func (m *JobManager) runLocalToRemote_GoRsync(job *Job, ctx context.Context) error {
	useLan := decideUseLan(job.Request)

	remoteHost, err := m.getHost(job.Plan.Dest.HostName)
	if err != nil {
		return err
	}
	d := dialFor(&remoteHost.Config, useLan)

	w := &jobLineWriter{job: job}

	clientArgs := buildRsyncArgs(&job.Request.Options)
	rsClient, err := rsyncclient.New(clientArgs, rsyncclient.WithSender(), rsyncclient.WithStderr(w))
	if err != nil {
		return fmt.Errorf("rsyncclient.New(sender): %w", err)
	}

	sshCli, err := sshDial(&remoteHost.Config, d)
	if err != nil {
		return err
	}
	defer sshCli.Close()

	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}
	go io.Copy(w, stderr)

	remoteServerArgs := rsClient.ServerCommandOptions(job.Plan.Dest.Path)
	remoteCmd := "rsync " + joinShellArgs(remoteServerArgs)

	job.mu.Lock()
	job.LogLines = append(job.LogLines, fmt.Sprintf("[go-rsync] ssh %s@%s:%d  %s", remoteHost.Config.User, d.Host, d.Port, remoteCmd))
	job.mu.Unlock()

	if err := sess.Start("sh -c " + shQuote(remoteCmd)); err != nil {
		return fmt.Errorf("start remote rsync server: %w", err)
	}

	rw := &struct {
		io.Reader
		io.Writer
	}{Reader: stdout, Writer: stdin}

	if _, err := rsClient.Run(ctx, rw, []string{job.Plan.Source.Path}); err != nil {
		_ = sess.Close()
		w.Flush()
		return fmt.Errorf("rsyncclient.Run(sender): %w", err)
	}

	if err := sess.Wait(); err != nil {
		w.Flush()
		return fmt.Errorf("remote rsync server exit: %w", err)
	}

	w.Flush()
	return nil
}

// ============================
// 3) remote -> local：Go 内置 SSH + gokrazy/rsyncclient（本机不需要 rsync/ssh）
// ============================
func (m *JobManager) runRemoteToLocal_GoRsync(job *Job, ctx context.Context) error {
	useLan := decideUseLan(job.Request)

	remoteHost, err := m.getHost(job.Plan.Source.HostName)
	if err != nil {
		return err
	}
	d := dialFor(&remoteHost.Config, useLan)

	w := &jobLineWriter{job: job}

	clientArgs := buildRsyncArgs(&job.Request.Options)
	rsClient, err := rsyncclient.New(clientArgs, rsyncclient.WithStderr(w))
	if err != nil {
		return fmt.Errorf("rsyncclient.New(receiver): %w", err)
	}

	sshCli, err := sshDial(&remoteHost.Config, d)
	if err != nil {
		return err
	}
	defer sshCli.Close()

	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}
	go io.Copy(w, stderr)

	remoteServerArgs := rsClient.ServerCommandOptions(job.Plan.Source.Path)
	remoteCmd := "cd ~ 2>/dev/null && command rsync " + joinShellArgs(remoteServerArgs)

	job.mu.Lock()
	job.LogLines = append(job.LogLines,
		fmt.Sprintf("[go-rsync] ssh %s@%s:%d  %s", remoteHost.Config.User, d.Host, d.Port, remoteCmd),
	)
	job.mu.Unlock()

	if err := sess.Start("sh -c " + shQuote(remoteCmd)); err != nil {
		return fmt.Errorf("start remote rsync server: %w", err)
	}

	rw := &struct {
		io.Reader
		io.Writer
	}{Reader: stdout, Writer: stdin}

	if _, err := rsClient.Run(ctx, rw, []string{job.Plan.Dest.Path}); err != nil {
		_ = sess.Close()
		w.Flush()
		return fmt.Errorf("rsyncclient.Run(receiver): %w", err)
	}

	if err := sess.Wait(); err != nil {
		w.Flush()
		return fmt.Errorf("remote rsync server exit: %w", err)
	}

	w.Flush()
	return nil
}

// ============================
// 4) remote <-> remote：一跳 SSH 到 execHost，在 execHost 上跑 rsync 命令行
//   - 支持不同 key：用 Go 内置 agent 转发，免 ssh-add（inner ssh 不写 -i，走转发 agent）:contentReference[oaicite:2]{index=2}
//   - 支持密码：inner ssh 用 sshpass -p 'xxx' ssh ...
//
// ============================
func (m *JobManager) runRemoteToRemote_OneHopSSH(job *Job, ctx context.Context) error {
	useLan := decideUseLan(job.Request)

	plan := job.Plan
	req := job.Request

	srcHost, err := m.getHost(plan.Source.HostName)
	if err != nil {
		return err
	}
	dstHost, err := m.getHost(plan.Dest.HostName)
	if err != nil {
		return err
	}
	execHost, err := m.getHost(plan.ExecHost)
	if err != nil {
		return err
	}

	// execHost 必须是 source 或 dest（你当前规划就是这样）
	execIsSource := plan.ExecHost == plan.Source.HostName
	execIsDest := plan.ExecHost == plan.Dest.HostName
	if !execIsSource && !execIsDest {
		return fmt.Errorf("execHost must be source or dest for now, got %q", plan.ExecHost)
	}

	// innerTarget = execHost 要连接的“另一端”
	var innerTarget *Host
	if execIsSource {
		innerTarget = dstHost
	} else {
		innerTarget = srcHost
	}
	
	// 连接 execHost（第一跳）
	execDial := dialFor(&execHost.Config, false)

	w := &jobLineWriter{job: job}
	job.mu.Lock()
	job.LogLines = append(job.LogLines,
		fmt.Sprintf("[remote-remote] first hop (control->execHost) %s@%s:%d (LAN=%v, forced WAN)",
			execHost.Config.User, execDial.Host, execDial.Port, useLan,
		),
	)
	job.mu.Unlock()

	sshCli, err := sshDial(&execHost.Config, execDial)
	if err != nil {
		return err
	}
	defer sshCli.Close()

	// 如果 innerTarget 需要 key，就把 key 加进内存 keyring，并转发到 execHost
	if innerTarget.Config.Auth == "private_key" && innerTarget.Config.KeyPath != "" {
		if err := forwardKeysToExecHost(sshCli, []string{innerTarget.Config.KeyPath}); err != nil {
			return err
		}
	}

	// 开 session，开启 agent forwarding（只有上面转发了 key 才需要）
	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	if innerTarget.Config.Auth == "private_key" && innerTarget.Config.KeyPath != "" {
		if err := agent.RequestAgentForwarding(sess); err != nil {
			return fmt.Errorf("RequestAgentForwarding: %w", err)
		}
	}

	stdout, _ := sess.StdoutPipe()
	stderr, _ := sess.StderrPipe()
	go io.Copy(w, stdout)
	go io.Copy(w, stderr)

	// 组 inner ssh / rsync 命令
	innerDial := dialFor(&innerTarget.Config, useLan)
	innerSSH := buildInnerSSHCommand(innerTarget.Config, innerDial)

	args := buildRsyncArgs(&req.Options)
	args = append(args, "--protect-args", "-e", innerSSH)

	var srcSpec, dstSpec string
	if execIsSource {
		// execHost=source：src 是本地路径；dst 是 user@host:path
		srcSpec = plan.Source.Path
		dstSpec = fmt.Sprintf("%s@%s:%s", innerTarget.Config.User, innerDial.Host, plan.Dest.Path)
	} else {
		// execHost=dest：src 是 user@host:path；dst 是本地路径
		srcSpec = fmt.Sprintf("%s@%s:%s", innerTarget.Config.User, innerDial.Host, plan.Source.Path)
		dstSpec = plan.Dest.Path
	}

	cmdStr := "rsync " + joinShellArgs(args) + " " + shQuote(srcSpec) + " " + shQuote(dstSpec)

	job.mu.Lock()
	job.LogLines = append(job.LogLines, "[remote-remote] "+cmdStr)
	job.mu.Unlock()

	if err := sess.Start("bash -lc " + shQuote(cmdStr)); err != nil {
		w.Flush()
		return fmt.Errorf("start remote command: %w", err)
	}
	err = sess.Wait()
	w.Flush()
	return err
}

// 把一组 keyPath 加载进内存 keyring，并 ForwardToAgent 到 sshCli（execHost）
// 注意：这只“把 agent 服务挂到 execHost 的 SSH 连接上”，真正启用要在 session 上 RequestAgentForwarding。
func forwardKeysToExecHost(sshCli *ssh.Client, keyPaths []string) error {
	keyring := agent.NewKeyring()
	for _, kp := range keyPaths {
		priv, err := readPrivateKeyObject(kp)
		if err != nil {
			return fmt.Errorf("read private key object %s: %w", kp, err)
		}
		if err := keyring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
			return fmt.Errorf("agent add key %s: %w", kp, err)
		}
	}
	if err := agent.ForwardToAgent(sshCli, keyring); err != nil {
		return fmt.Errorf("ForwardToAgent: %w", err)
	}
	return nil
}

// ===== SSH Dial（Go 内置，支持 key 或 password）=====
func sshDial(cfg *HostConfig, d DialTarget) (*ssh.Client, error) {
	addr := net.JoinHostPort(d.Host, strconv.Itoa(d.Port))

	auths, err := buildSSHAuthMethods(cfg)
	if err != nil {
		return nil, err
	}

	cc := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	netConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	cconn, chans, reqs, err := ssh.NewClientConn(netConn, addr, cc)
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	return ssh.NewClient(cconn, chans, reqs), nil
}

func buildSSHAuthMethods(cfg *HostConfig) ([]ssh.AuthMethod, error) {
	switch cfg.Auth {
	case "password":
		return []ssh.AuthMethod{ssh.Password(cfg.Password)}, nil
	case "private_key":
		signer, err := readSigner(cfg.KeyPath)
		if err != nil {
			return nil, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return nil, fmt.Errorf("unsupported auth: %q", cfg.Auth)
	}
}

func readSigner(keyPath string) (ssh.Signer, error) {
	if keyPath == "" {
		return nil, errors.New("empty keyPath")
	}
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	s, err := ssh.ParsePrivateKey(b)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// 为了 agent.AddedKey，需要“私钥对象”（rsa.PrivateKey / ed25519.PrivateKey / ecdsa.PrivateKey）
func readPrivateKeyObject(keyPath string) (any, error) {
	if keyPath == "" {
		return nil, fmt.Errorf("empty keyPath")
	}
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	// 关键点：ParseRawPrivateKey 返回 *rsa.PrivateKey / ed25519.PrivateKey / *ecdsa.PrivateKey
	priv, err := ssh.ParseRawPrivateKey(b)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

// inner ssh：execHost -> 另一台（支持：key 走 agent；password 走 sshpass）
func buildInnerSSHCommand(target HostConfig, d DialTarget) string {
	base := []string{}

	// password：要求 execHost 上有 sshpass
	if target.Auth == "password" && target.Password != "" {
		base = append(base, "sshpass", "-p", shQuote(target.Password), "ssh")
	} else {
		base = append(base, "ssh")
		// key：不要 -i（因为 execHost 没你的 key 文件），走“agent 转发”
		base = append(base, "-o", "PreferredAuthentications=publickey")
	}

	if d.Port > 0 && d.Port != 22 {
		base = append(base, "-p", strconv.Itoa(d.Port))
	}

	// 连接稳定性（你之前 job 一直 running，很多时候就是卡在 SSH/密码提示上）
	base = append(base,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=2",
	)

	return strings.Join(base, " ")
}

// ===== shell quoting helpers =====
func shQuote(s string) string {
	// 单引号包裹，内部 ' -> '"'"'
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func joinShellArgs(args []string) string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, shQuote(a))
	}
	return strings.Join(out, " ")
}
