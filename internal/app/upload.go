package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// UploadLocalFile
// localFile: 已经在本机落盘的临时文件路径
// hostName: YAML 里的 name 或 "local"
// dstDir: 目标目录（GUI 里填的 path）
// fileName: 原始文件名（用于目标文件名）
// relPath: 相对 dstDir 的路径，可以是 "a.txt" 或 "sub/dir/a.txt"
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

	// 远程：先 mkdir -p 目录，再 rsync 文件
	mkdirCmd := fmt.Sprintf(`mkdir -p %q`, finalDirRemote)
	if _, err := runSSH(h, mkdirCmd); err != nil {
		return fmt.Errorf("remote mkdir %q failed: %w", finalDirRemote, err)
	}

	// rsync 上传：目标一定要是“完整文件路径”，否则会保留临时文件名 upload-xxxx
	args := []string{"-av", "--protect-args"}

	sshArgs := []string{"ssh", "-p", strconv.Itoa(h.Config.Port)}
	if h.Config.Auth == "private_key" && h.Config.KeyPath != "" {
		sshArgs = append(sshArgs, "-i", h.Config.KeyPath)
	}
	sshArgs = append(sshArgs,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	)
	sshCmd := strings.Join(sshArgs, " ")

	args = append(args, "-e", sshCmd)
	args = append(args, localFile)

	remoteDstPath := joinPosix(finalDirRemote, filepath.ToSlash(baseName))
	dstSpec := fmt.Sprintf("%s@%s:%s", h.Config.User, h.Config.Host, remoteDstPath)
	args = append(args, dstSpec)

	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync upload failed: %w, output: %s", err, string(out))
	}

	_ = os.Remove(localFile)
	return nil
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
