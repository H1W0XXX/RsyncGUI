package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gokrazy/rsync/rsyncclient"
	"github.com/google/uuid"
)

// UploadLocalFile: 拖拽上传（本机->远端用 Go rsyncclient，不再 exec 本机 rsync）
func (a *App) UploadLocalFile(hostName, dstDir, relPath, localFile string) error {
	h, ok := a.Hosts.Get(hostName)
	if !ok {
		return fmt.Errorf("unknown host %q", hostName)
	}

	if relPath == "" {
		relPath = filepath.Base(localFile)
	}

	relDir := filepath.Dir(relPath)
	baseName := filepath.Base(relPath)

	// 本机/远程分别拼路径（避免 Windows 上 filepath.Join 和 posix 混用）
	finalDirLocal := dstDir
	finalDirRemote := dstDir
	if relDir != "." && relDir != "" {
		finalDirLocal = filepath.Join(dstDir, relDir)
		finalDirRemote = joinPosix(dstDir, filepath.ToSlash(relDir))
	}

	// local 直接移动
	if h.IsLocal {
		if err := os.MkdirAll(finalDirLocal, 0o755); err != nil {
			return fmt.Errorf("mkdir local dst: %w", err)
		}
		dstPath := filepath.Join(finalDirLocal, baseName)
		if err := os.Rename(localFile, dstPath); err != nil {
			return fmt.Errorf("move file to dst: %w", err)
		}
		return nil
	}

	// ===== 远端：mkdir -p 目录 =====
	mkdirCmd := fmt.Sprintf(`mkdir -p %q`, finalDirRemote)
	if _, err := runSSH(h, mkdirCmd); err != nil {
		return fmt.Errorf("remote mkdir %q failed: %w", finalDirRemote, err)
	}

	// ===== 关键：把临时文件“变成”正确文件名，否则远端会保留 upload-xxxx =====
	// 用独立 staging 目录避免同名冲突，同时确保 rsync 看到的 basename 就是目标文件名。
	stageDir := filepath.Join(filepath.Dir(localFile), "rsyncgui-upload-"+uuid.New().String())
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return fmt.Errorf("mkdir stage dir: %w", err)
	}
	stagedPath := filepath.Join(stageDir, baseName)

	if err := os.Rename(localFile, stagedPath); err != nil {
		// rename 失败（比如跨盘）就 copy 一份再删原文件
		if err2 := copyFile(stagedPath, localFile); err2 != nil {
			_ = os.RemoveAll(stageDir)
			return fmt.Errorf("stage file failed: rename=%v copy=%v", err, err2)
		}
		_ = os.Remove(localFile)
	}

	// ===== 用 Go rsyncclient 推送到远端目录 =====
	ctx := context.Background()

	clientArgs := []string{"-av", "--protect-args"} // 你要额外参数也可以继续 append
	rsClient, err := rsyncclient.New(clientArgs, rsyncclient.WithSender())
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("rsyncclient.New(sender): %w", err)
	}

	// 复用你项目里现成的 sshDial
	d := DialTarget{Host: h.Config.Host, Port: h.Config.Port}
	if d.Port == 0 {
		d.Port = 22
	}
	sshCli, err := sshDial(&h.Config, d)
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return err
	}
	defer sshCli.Close()

	sess, err := sshCli.NewSession()
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return err
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		_ = os.RemoveAll(stageDir)
		return err
	}
	// 这里你可以接你已有的 jobLineWriter；UploadLocalFile 没 job，就先丢弃或打印
	go io.Copy(io.Discard, stderr)

	remoteServerArgs := rsClient.ServerCommandOptions(finalDirRemote)
	remoteCmd := "cd ~ 2>/dev/null && command rsync " + joinShellArgs(remoteServerArgs)

	if err := sess.Start("sh -c " + shQuote(remoteCmd)); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("start remote rsync server: %w", err)
	}

	rw := &struct {
		io.Reader
		io.Writer
	}{Reader: stdout, Writer: stdin}

	if _, err := rsClient.Run(ctx, rw, []string{stagedPath}); err != nil {
		_ = sess.Close()
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("rsyncclient.Run(sender): %w", err)
	}

	if err := sess.Wait(); err != nil {
		_ = os.RemoveAll(stageDir)
		return fmt.Errorf("remote rsync server exit: %w", err)
	}

	_ = os.RemoveAll(stageDir)
	return nil
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func joinPosix(dir, rel string) string {
	dir = strings.TrimRight(dir, "/")
	rel = strings.TrimLeft(rel, "/")
	if rel == "" || rel == "." {
		return dir
	}
	if dir == "" {
		return rel
	}
	return dir + "/" + rel
}
