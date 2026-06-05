package app

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gokrazy/rsync/rsyncclient"
	"golang.org/x/crypto/ssh"
)

type remoteUploadTransport string

const (
	remoteUploadTransportRsync remoteUploadTransport = "rsync"
	remoteUploadTransportSCP   remoteUploadTransport = "scp"
)

func probeRemoteUploadTransport(ctx context.Context, sshCli *ssh.Client, w *jobLineWriter) (remoteUploadTransport, error) {
	rsyncCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	rsyncErr := probeRemoteRsyncBlackhole(rsyncCtx, sshCli, w)
	cancel()
	if rsyncErr == nil {
		w.appendLine("[probe] rsync blackhole upload ok")
		return remoteUploadTransportRsync, nil
	}
	w.appendLine("[probe] rsync blackhole upload failed: " + rsyncErr.Error())

	scpCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	scpErr := probeRemoteScpBlackhole(scpCtx, sshCli, w)
	cancel()
	if scpErr == nil {
		w.appendLine("[probe] scp blackhole upload ok; falling back to scp")
		return remoteUploadTransportSCP, nil
	}
	w.appendLine("[probe] scp blackhole upload failed: " + scpErr.Error())

	return "", fmt.Errorf("remote upload probe failed: rsync 123.txt blackhole failed: %v; scp 123.txt blackhole failed: %w", rsyncErr, scpErr)
}

func probeRemoteRsyncBlackhole(ctx context.Context, sshCli *ssh.Client, w *jobLineWriter) error {
	localDir, err := os.MkdirTemp("", "rsyncgui-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(localDir)

	probeFile := filepath.Join(localDir, "123.txt")
	if err := os.WriteFile(probeFile, []byte("123\n"), 0o644); err != nil {
		return err
	}

	rsClient, err := rsyncclient.New([]string{"-r"}, rsyncclient.WithSender(), rsyncclient.WithStderr(w))
	if err != nil {
		return fmt.Errorf("rsyncclient.New(probe): %w", err)
	}

	remoteDir := fmt.Sprintf("/tmp/rsyncgui-rsync-blackhole-%d", time.Now().UnixNano())
	remoteServerArgs := rsClient.ServerCommandOptions(remoteDir + "/")
	remoteCmd := fmt.Sprintf(
		"rm -rf %s && mkdir -p %s && trap 'rm -rf %s' EXIT && command rsync %s",
		shQuote(remoteDir),
		shQuote(remoteDir),
		shQuote(remoteDir),
		joinShellArgs(remoteServerArgs),
	)

	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stopCancel := closeSessionOnCancel(ctx, sess)
	defer stopCancel()

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

	w.appendLine("[probe] rsync 123.txt -> remote blackhole")
	if err := sess.Start("sh -c " + shQuote(remoteCmd)); err != nil {
		return fmt.Errorf("start remote rsync probe: %w", err)
	}

	rw := &struct {
		io.Reader
		io.Writer
	}{Reader: stdout, Writer: stdin}

	if _, err := rsClient.Run(ctx, rw, []string{probeFile}); err != nil {
		_ = sess.Close()
		w.Flush()
		return fmt.Errorf("rsyncclient.Run(probe): %w", err)
	}

	if err := waitSession(ctx, sess); err != nil {
		w.Flush()
		return fmt.Errorf("remote rsync probe exit: %w", err)
	}
	w.Flush()
	return nil
}

func probeRemoteScpBlackhole(ctx context.Context, sshCli *ssh.Client, w *jobLineWriter) error {
	w.appendLine("[probe] scp 123.txt -> /dev/null")
	return scpSendBytes(ctx, sshCli, "/dev/null", "123.txt", []byte("123\n"), w)
}

func scpSendBytes(ctx context.Context, sshCli *ssh.Client, remoteTarget, name string, data []byte, w *jobLineWriter) error {
	if err := validateScpName(name); err != nil {
		return err
	}

	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stopCancel := closeSessionOnCancel(ctx, sess)
	defer stopCancel()

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

	if err := sess.Start("scp -t " + shQuote(remoteTarget)); err != nil {
		return fmt.Errorf("start remote scp sink: %w", err)
	}

	ack := bufio.NewReader(stdout)
	if err := readScpAck(ack); err != nil {
		_ = sess.Close()
		return err
	}
	if _, err := fmt.Fprintf(stdin, "C0644 %d %s\n", len(data), name); err != nil {
		_ = sess.Close()
		return err
	}
	if err := readScpAck(ack); err != nil {
		_ = sess.Close()
		return err
	}
	if _, err := stdin.Write(data); err != nil {
		_ = sess.Close()
		return err
	}
	if _, err := stdin.Write([]byte{0}); err != nil {
		_ = sess.Close()
		return err
	}
	if err := readScpAck(ack); err != nil {
		_ = sess.Close()
		return err
	}
	if err := stdin.Close(); err != nil {
		_ = sess.Close()
		return err
	}
	return waitSession(ctx, sess)
}

func scpSendLocalPath(ctx context.Context, sshCli *ssh.Client, localPath, remoteDir string, sendContents bool, w *jobLineWriter) error {
	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stopCancel := closeSessionOnCancel(ctx, sess)
	defer stopCancel()

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

	cmd := "mkdir -p " + shQuote(remoteDir) + " && scp -r -t " + shQuote(remoteDir)
	if err := sess.Start("sh -c " + shQuote(cmd)); err != nil {
		return fmt.Errorf("start remote scp sink: %w", err)
	}

	ack := bufio.NewReader(stdout)
	if err := readScpAck(ack); err != nil {
		_ = sess.Close()
		return err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		_ = sess.Close()
		return err
	}
	if info.IsDir() && sendContents {
		entries, err := os.ReadDir(localPath)
		if err != nil {
			_ = sess.Close()
			return err
		}
		for _, entry := range entries {
			if err := scpSendPathEntry(stdin, ack, filepath.Join(localPath, entry.Name())); err != nil {
				_ = sess.Close()
				return err
			}
		}
	} else if err := scpSendPathEntry(stdin, ack, localPath); err != nil {
		_ = sess.Close()
		return err
	}

	if err := stdin.Close(); err != nil {
		_ = sess.Close()
		return err
	}
	return waitSession(ctx, sess)
}

func tarGzipSendLocalPath(ctx context.Context, sshCli *ssh.Client, localPath, remoteDir string, sendContents bool, w *jobLineWriter) error {
	sess, err := sshCli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stopCancel := closeSessionOnCancel(ctx, sess)
	defer stopCancel()

	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}
	go io.Copy(w, stderr)

	cmd := "mkdir -p " + shQuote(remoteDir) + " && tar -xzf - -C " + shQuote(remoteDir)
	if err := sess.Start("sh -c " + shQuote(cmd)); err != nil {
		return fmt.Errorf("start remote tar sink: %w", err)
	}

	writeErr := writeTarGzipPath(stdin, localPath, sendContents)
	closeErr := stdin.Close()
	if writeErr != nil {
		_ = sess.Close()
		return writeErr
	}
	if closeErr != nil {
		_ = sess.Close()
		return closeErr
	}
	return waitSession(ctx, sess)
}

func writeTarGzipPath(dst io.Writer, localPath string, sendContents bool) error {
	gzw := gzip.NewWriter(dst)
	tw := tar.NewWriter(gzw)

	closeWriters := func() error {
		if err := tw.Close(); err != nil {
			_ = gzw.Close()
			return err
		}
		return gzw.Close()
	}

	info, err := os.Stat(localPath)
	if err != nil {
		_ = closeWriters()
		return err
	}

	if info.IsDir() && sendContents {
		entries, err := os.ReadDir(localPath)
		if err != nil {
			_ = closeWriters()
			return err
		}
		for _, entry := range entries {
			if err := writeTarPathEntry(tw, filepath.Join(localPath, entry.Name()), entry.Name()); err != nil {
				_ = closeWriters()
				return err
			}
		}
		return closeWriters()
	}

	if err := writeTarPathEntry(tw, localPath, filepath.Base(localPath)); err != nil {
		_ = closeWriters()
		return err
	}
	return closeWriters()
}

func writeTarPathEntry(tw *tar.Writer, path, archiveName string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("compressed fallback does not support symlinks: %s", path)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return fmt.Errorf("compressed fallback only supports regular files and directories: %s", path)
	}

	archiveName = filepath.ToSlash(archiveName)
	archiveName = strings.TrimPrefix(archiveName, "/")
	if archiveName == "" || archiveName == "." || archiveName == ".." || strings.HasPrefix(archiveName, "../") {
		return fmt.Errorf("invalid archive path %q", archiveName)
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = archiveName

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childArchiveName := filepath.ToSlash(filepath.Join(archiveName, entry.Name()))
			if err := writeTarPathEntry(tw, filepath.Join(path, entry.Name()), childArchiveName); err != nil {
				return err
			}
		}
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func scpSendPathEntry(dst io.Writer, ack *bufio.Reader, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	name := filepath.Base(path)
	if err := validateScpName(name); err != nil {
		return err
	}

	mode := info.Mode().Perm()
	if info.IsDir() {
		if _, err := fmt.Fprintf(dst, "D%04o 0 %s\n", mode, name); err != nil {
			return err
		}
		if err := readScpAck(ack); err != nil {
			return err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := scpSendPathEntry(dst, ack, filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(dst, "E\n"); err != nil {
			return err
		}
		return readScpAck(ack)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("scp fallback only supports regular files and directories: %s", path)
	}

	if _, err := fmt.Fprintf(dst, "C%04o %d %s\n", mode, info.Size(), name); err != nil {
		return err
	}
	if err := readScpAck(ack); err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(dst, f); err != nil {
		return err
	}
	if _, err := dst.Write([]byte{0}); err != nil {
		return err
	}
	return readScpAck(ack)
}

func readScpAck(r *bufio.Reader) error {
	b, err := r.ReadByte()
	if err != nil {
		return err
	}
	switch b {
	case 0:
		return nil
	case 1, 2:
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			line = fmt.Sprintf("scp returned code %d", b)
		}
		return fmt.Errorf("remote scp error: %s", line)
	default:
		return fmt.Errorf("unexpected scp ack byte: %d", b)
	}
}

func validateScpName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid scp name %q", name)
	}
	if strings.ContainsAny(name, "\x00\r\n/\\") {
		return fmt.Errorf("scp name contains unsafe characters: %q", name)
	}
	return nil
}

func waitSession(ctx context.Context, sess *ssh.Session) error {
	done := make(chan error, 1)
	go func() {
		done <- sess.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = sess.Close()
		err := <-done
		if err != nil {
			return fmt.Errorf("%w; session closed with: %v", ctx.Err(), err)
		}
		return ctx.Err()
	}
}

func closeSessionOnCancel(ctx context.Context, sess *ssh.Session) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sess.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func (w *jobLineWriter) appendLine(line string) {
	w.job.mu.Lock()
	w.job.LogLines = append(w.job.LogLines, line)
	w.job.mu.Unlock()
}
