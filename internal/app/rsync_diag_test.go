//go:build rsyncdiag

package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/gokrazy/rsync/rsyncclient"
	"golang.org/x/crypto/ssh"
)

func TestRsyncDiagSizeMatrix(t *testing.T) {
	hostName := getenvDefault("RSYNCGUI_DIAG_HOST", "日本甲骨文2")
	hostsPath := getenvDefault("RSYNCGUI_DIAG_HOSTS", findRepoFile("hosts.yaml"))
	destBase := getenvDefault("RSYNCGUI_DIAG_DEST", fmt.Sprintf("/tmp/rsyncgui-diag-size-%d", time.Now().UnixNano()))

	hosts, err := LoadHosts(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg *HostConfig
	for i := range hosts {
		if hosts[i].Name == hostName {
			cfg = &hosts[i]
			break
		}
	}
	if cfg == nil {
		t.Fatalf("host %q not found", hostName)
	}

	d := dialFor(cfg, false)
	sshCli, err := sshDial(cfg, d, false)
	if err != nil {
		t.Fatal(err)
	}
	defer sshCli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	localDir := t.TempDir()
	for _, size := range []int{1024, 16 * 1024, 32*1024 - 1, 32 * 1024, 32*1024 + 1, 64 * 1024, 256 * 1024, 300 * 1024} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			source := filepath.Join(localDir, fmt.Sprintf("file-%d.bin", size))
			data := make([]byte, size)
			for i := range data {
				data[i] = byte(i)
			}
			if err := os.WriteFile(source, data, 0666); err != nil {
				t.Fatal(err)
			}
			dest := path.Join(destBase, fmt.Sprintf("%d", size)) + "/"
			err := diagGoRsyncUpload(ctx, sshCli, []string{"-r"}, source, dest, t.Logf)
			if err != nil {
				t.Fatalf("upload failed: %v", err)
			}
		})
	}
}

func TestRsyncDiagMatrix(t *testing.T) {
	hostName := getenvDefault("RSYNCGUI_DIAG_HOST", "日本甲骨文2")
	hostsPath := getenvDefault("RSYNCGUI_DIAG_HOSTS", findRepoFile("hosts.yaml"))
	source := getenvDefault("RSYNCGUI_DIAG_SOURCE", `D:\go\go-control\bin\ctrlsolve-linux-amd64`)
	destBase := getenvDefault("RSYNCGUI_DIAG_DEST", fmt.Sprintf("/tmp/rsyncgui-diag-%d", time.Now().UnixNano()))

	hosts, err := LoadHosts(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg *HostConfig
	for i := range hosts {
		if hosts[i].Name == hostName {
			cfg = &hosts[i]
			break
		}
	}
	if cfg == nil {
		t.Fatalf("host %q not found", hostName)
	}

	d := dialFor(cfg, false)
	sshCli, err := sshDial(cfg, d, false)
	if err != nil {
		t.Fatal(err)
	}
	defer sshCli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for _, args := range [][]string{
		{"-r"},
		{"-rz"},
		{"-rt"},
		{"-rp"},
		{"-rltp"},
		{"-rltpz"},
	} {
		t.Run(fmt.Sprintf("%v", args), func(t *testing.T) {
			dest := path.Join(destBase, args[0]) + "/"
			err := diagGoRsyncUpload(ctx, sshCli, args, source, dest, t.Logf)
			if err != nil {
				t.Fatalf("upload failed: %v", err)
			}
		})
	}
}

func getenvDefault(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func findRepoFile(name string) string {
	wd, err := os.Getwd()
	if err != nil {
		return name
	}
	for {
		candidate := filepath.Join(wd, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return name
		}
		wd = parent
	}
}

func diagGoRsyncUpload(ctx context.Context, sshCli interface {
	NewSession() (*ssh.Session, error)
}, args []string, source, dest string, logf func(string, ...any)) error {
	rsClient, err := rsyncclient.New(args, rsyncclient.WithSender())
	if err != nil {
		return fmt.Errorf("rsyncclient.New: %w", err)
	}

	sess, err := sshCli.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
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
	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			logf("remote stderr: %s", string(b))
		}
	}()

	remoteArgs := forceRemoteRsyncProtocol(rsClient.ServerCommandOptions(dest))
	remoteCmd := "mkdir -p " + shQuote(dest) + " && command rsync " + joinShellArgs(remoteArgs)
	logf("remote command: %s", remoteCmd)
	if err := sess.Start("sh -c " + shQuote(remoteCmd)); err != nil {
		return fmt.Errorf("start remote rsync: %w", err)
	}

	rw := &struct {
		io.Reader
		io.Writer
	}{Reader: stdout, Writer: stdin}

	if _, err := rsClient.Run(ctx, rw, []string{source}); err != nil {
		_ = sess.Close()
		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		waitErr := waitSession(waitCtx, sess)
		cancel()
		if waitErr != nil {
			logf("remote wait after client error: %v", waitErr)
		}
		return fmt.Errorf("rsyncclient.Run: %w", err)
	}
	if err := sess.Wait(); err != nil {
		return fmt.Errorf("remote rsync exit: %w", err)
	}
	return nil
}
