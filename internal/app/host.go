package app

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Host struct {
	Config  HostConfig
	IsLocal bool
}

type HostRegistry struct {
	mu     sync.RWMutex
	byName map[string]*Host
	local  *Host
}

func NewHostRegistry(configs []HostConfig) (*HostRegistry, error) {
	reg := &HostRegistry{
		byName: make(map[string]*Host),
	}

	// 虚拟 local host
	localHost := &Host{
		Config: HostConfig{
			Name: "local",
		},
		IsLocal: true,
	}
	reg.byName["local"] = localHost
	reg.local = localHost

	for _, c := range configs {
		hc := c
		h := &Host{
			Config:  hc,
			IsLocal: false,
		}
		if hc.Name == "" {
			return nil, fmt.Errorf("host name empty in config")
		}
		if _, ok := reg.byName[hc.Name]; ok {
			return nil, fmt.Errorf("duplicate host name: %s", hc.Name)
		}
		reg.byName[hc.Name] = h
	}
	return reg, nil
}

func (r *HostRegistry) Get(name string) (*Host, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byName[name]
	return h, ok
}

func (r *HostRegistry) All() []*Host {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Host, 0, len(r.byName))
	for _, h := range r.byName {
		out = append(out, h)
	}
	return out
}

func (r *HostRegistry) Local() *Host {
	return r.local
}

func runSSHGo(h *Host, remoteCmd string) (string, error) {
	cfg := h.Config

	auths, err := sshAuthMethods(&cfg)
	if err != nil {
		return "", err
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := ssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	sess, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	// PTY：可选，失败也别影响逻辑（但打印出来方便排查）
	if err := sess.RequestPty("xterm", 80, 40, ssh.TerminalModes{}); err != nil {
		fmt.Printf("[ssh] host=%s pty denied: %v\n", h.Config.Name, err)
	}

	// ✅ 正常模式：不加任何哨兵，避免污染业务输出
	cmd := "sh -c " + shellQuote(remoteCmd)
	fmt.Printf("[ssh] host=%s cmd=%q\n", h.Config.Name, cmd)

	b, err := sess.CombinedOutput(cmd)
	out := string(b)

	// ✅ 永远打印 err（你之前排障痛点）
	if err != nil {
		fmt.Printf("[ssh] host=%s ERR=%v out=%q\n", h.Config.Name, err, out)
		return out, fmt.Errorf("ssh run failed: %w", err)
	}

	fmt.Printf("[ssh] host=%s OK out=%q\n", h.Config.Name, out)
	return out, nil
}

func sshAuthMethods(cfg *HostConfig) ([]ssh.AuthMethod, error) {
	var out []ssh.AuthMethod

	// password
	if cfg.Auth == "password" && cfg.Password != "" {
		out = append(out, ssh.Password(cfg.Password))
		return out, nil
	}

	// private key
	if cfg.Auth == "private_key" && cfg.KeyPath != "" {
		keyBytes, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read keyPath %s: %w", cfg.KeyPath, err)
		}

		// 兼容：未加密 key
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			// 如果你的 key 有 passphrase，这里需要 ParsePrivateKeyWithPassphrase
			return nil, fmt.Errorf("parse private key failed: %w", err)
		}
		out = append(out, ssh.PublicKeys(signer))
		return out, nil
	}

	return out, nil
}

// shellQuote: 用单引号包住，内部单引号安全转义
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// runSSH 在本机执行 ssh 命令，返回 stdout+stderr
// 注意：这里只是底层工具，真正 rsync 命令会在 executor 里写
func runSSH(h *Host, remoteCmd string) (string, error) {
	if h.IsLocal {
		return runLocalShell(remoteCmd)
	}
	return runSSHGo(h, remoteCmd)
}

// 简单本地 shell 执行（兼容 Windows / *nix）
func runLocalShell(cmdStr string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	} else {
		cmd = exec.Command("bash", "-lc", cmdStr)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("local shell failed: %w; output=%s", err, buf.String())
	}
	return strings.TrimSpace(buf.String()), nil
}

func (h *Host) Run(cmdStr string) (string, error) {
	if h.IsLocal {
		out, err := exec.Command("bash", "-lc", cmdStr).CombinedOutput()
		return string(out), err
	}
	return runSSHGo(h, cmdStr)
}
