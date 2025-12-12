package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FSEntry 表示目录里的一个条目
type FSEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	MTime int64  `json:"mtime,omitempty"` // unix seconds
	Size  int64  `json:"size,omitempty"`  // bytes
}

// FSListResult 列目录结果
type FSListResult struct {
	CWD     string    `json:"cwd"`     // 当前目录
	Entries []FSEntry `json:"entries"` // 文件/子目录列表
}

// ListDir 列出某个 host 上某个 path 下的目录内容
func (a *App) ListDir(hostName, path string) (*FSListResult, error) {
	h, ok := a.Hosts.Get(hostName)
	if !ok {
		return nil, fmt.Errorf("unknown host %q", hostName)
	}

	if h.IsLocal {
		entries, err := listDirLocal(path)
		if err != nil {
			return nil, err
		}
		return &FSListResult{CWD: path, Entries: entries}, nil
	}

	return listDirRemotePython(h, path)
}

func parsePyResult(tag, out string, err error) (string, error) {
	s := strings.ReplaceAll(out, "\r\n", "\n")
	rc := ""
	if i := strings.LastIndex(s, "__RC__"); i >= 0 {
		rc = strings.TrimSpace(s[i+6:])
		s = strings.TrimSpace(s[:i])
	} else {
		s = strings.TrimSpace(s)
	}
	if err != nil {
		return "", fmt.Errorf("%s: ssh err=%v out=%q rc=%q", tag, err, s, rc)
	}
	if rc != "" && rc != "0" {
		return "", fmt.Errorf("%s: remote rc=%s out=%q", tag, rc, s)
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%s: empty output rc=%q", tag, rc)
	}
	return strings.TrimSpace(s), nil
}

func runRemotePy(h *Host, code string) (string, error) {
	// 把 stderr 合并进 stdout，并回显返回码
	cmd := "python3 -c " + shQuote(code) + " 2>&1; echo __RC__$?"
	out, err := runSSH(h, cmd)

	// 解析 rc
	s := strings.ReplaceAll(out, "\r\n", "\n")
	rc := ""
	if i := strings.LastIndex(s, "__RC__"); i >= 0 {
		rc = strings.TrimSpace(s[i+6:])
		s = strings.TrimSpace(s[:i])
	} else {
		s = strings.TrimSpace(s)
	}

	if err != nil {
		return "", fmt.Errorf("ssh err=%v rc=%q out=%q", err, rc, s)
	}
	if rc != "" && rc != "0" {
		return "", fmt.Errorf("remote python rc=%s out=%q", rc, s)
	}
	return strings.TrimSpace(s), nil
}

func getRemoteHomeByPython(h *Host) (string, error) {
	out, err := runRemotePy(h, `import os; print(os.path.expanduser("~"), end="")`)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("empty home from python")
	}
	return out, nil
}

// HomeDir: 获取主目录 (~) 的绝对路径并列目录
func (a *App) HomeDir(hostName string) (*FSListResult, error) {
	h, ok := a.Hosts.Get(hostName)
	if !ok {
		return nil, fmt.Errorf("unknown host %q", hostName)
	}

	if h.IsLocal {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return a.ListDir(hostName, home)
	}

	home, err := getRemoteHomeByPython(h)
	if err != nil {
		return nil, err
	}
	return a.ListDir(hostName, home)
}

// ========= 下面是本机/远程各自的实现 =========

// 本机目录遍历：完全用 Go 标准库，兼容 Windows / Linux / Mac
func listDirLocal(path string) ([]FSEntry, error) {
	des, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	out := make([]FSEntry, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue
		}
		e := FSEntry{
			Name:  de.Name(),
			IsDir: de.IsDir(),
			MTime: info.ModTime().Unix(),
		}
		if !de.IsDir() {
			e.Size = info.Size()
		} else {
			e.Size = 0
		}
		out = append(out, e)
	}
	return out, nil
}

func listDirRemotePython(h *Host, path string) (*FSListResult, error) {
	// 远端 python 输出：三行（BEGIN / base64(json) / END）
	// 增加字段：mtime(秒) / size(字节, 目录给 0)
	py := fmt.Sprintf(
		`import os,json,base64; p=%q; items=[]; 
for n in os.listdir(p):
  fp=os.path.join(p,n)
  try:
    st=os.stat(fp, follow_symlinks=False)
    isdir=os.path.isdir(fp)
    items.append({"name":n,"isDir":isdir,"mtime":int(st.st_mtime),"size":(0 if isdir else int(st.st_size))})
  except Exception:
    continue
b=base64.b64encode(json.dumps(items,separators=(",",":")).encode()).decode(); 
print("__PY_BEGIN__"); print(b); print("__PY_END__")`,
		path,
	)

	// 不要 heredoc；stderr 合并，方便排错
	out, err := runSSH(h, "python3 -c "+shQuote(py)+" 2>&1")
	if err != nil {
		return nil, fmt.Errorf("remote listdir ssh failed: %w; out=%q", err, out)
	}

	// 提取 payload（不怕 banner/MOTD）
	b64, e := extractBetweenMarkers(out, "__PY_BEGIN__", "__PY_END__")
	if e != nil {
		return nil, fmt.Errorf("remote listdir markers not found: %v; raw=%q", e, out)
	}

	data, e := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if e != nil {
		return nil, fmt.Errorf("remote listdir base64 decode failed: %v; b64=%q; raw=%q", e, b64, out)
	}

	var entries []FSEntry
	if e := json.Unmarshal(data, &entries); e != nil {
		return nil, fmt.Errorf("remote listdir json unmarshal failed: %v; json=%q", e, string(data))
	}

	return &FSListResult{
		CWD:     path,
		Entries: entries,
	}, nil
}

func extractBetweenMarkers(raw, begin, end string) (string, error) {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(s, "\n")

	in := false
	var payload []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		t := strings.TrimSpace(line)
		if t == begin {
			in = true
			continue
		}
		if t == end {
			in = false
			break
		}
		if in && t != "" {
			payload = append(payload, line)
		}
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("no payload")
	}
	return strings.Join(payload, "\n"), nil
}
