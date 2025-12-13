package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PathInfo struct {
	Exists   bool     `json:"exists"`
	IsDir    bool     `json:"isDir"`
	Readable bool     `json:"readable"`
	Writable bool     `json:"writable"`
	Items    []string `json:"items"`
	RawLS    string   `json:"rawLS"`
}

func (a *App) CheckPath(hostName, path string) (*PathInfo, error) {
	h, ok := a.Hosts.Get(hostName)
	if !ok {
		return nil, fmt.Errorf("unknown host %q", hostName)
	}

	info := &PathInfo{}

	// ✅ local：不用 shell，直接用 Go
	if h.IsLocal {
		st, err := os.Stat(path)
		if err != nil {
			// 不存在/无权限都返回 Exists=false（跟你现在 test -e 的语义接近）
			return info, nil
		}
		info.Exists = true
		info.IsDir = st.IsDir()

		// Readable
		if info.IsDir {
			_, err = os.ReadDir(path)
			info.Readable = (err == nil)
		} else {
			f, err := os.Open(path)
			if err == nil {
				_ = f.Close()
				info.Readable = true
			}
		}

		// Writable：目录 -> 创建临时文件；文件 -> 以写方式打开
		if info.IsDir {
			tmp := filepath.Join(path, fmt.Sprintf(".rsync_gui_test_%d", time.Now().UnixNano()))
			f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err == nil {
				_ = f.Close()
				_ = os.Remove(tmp)
				info.Writable = true
			}
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
			if err == nil {
				_ = f.Close()
				info.Writable = true
			}
		}

		// Items：如果是目录，列出条目（和 ls -1 类似）
		if info.IsDir {
			des, err := os.ReadDir(path)
			if err == nil {
				// RawLS 给个简单的 "\n" 拼接，前端如果只是展示/调试够用了
				var b strings.Builder
				for _, de := range des {
					name := de.Name()
					info.Items = append(info.Items, name)
					b.WriteString(name)
					b.WriteByte('\n')
				}
				info.RawLS = b.String()
			}
		}
		return info, nil
	}

	// ✅ remote：保留你原来的 shell 逻辑（Linux 上可用）
	_, err := h.Run(fmt.Sprintf(`test -e "%s"`, path))
	info.Exists = err == nil
	if !info.Exists {
		return info, nil
	}

	_, err = h.Run(fmt.Sprintf(`test -d "%s"`, path))
	info.IsDir = err == nil

	_, err = h.Run(fmt.Sprintf(`test -r "%s"`, path))
	info.Readable = err == nil

	_, err = h.Run(fmt.Sprintf(`test -w "%s"`, path))
	info.Writable = err == nil

	if info.IsDir {
		out, err := h.Run(fmt.Sprintf(`ls -1 "%s"`, path))
		if err == nil {
			info.RawLS = out
			lines := strings.Split(out, "\n")
			for _, ln := range lines {
				ln = strings.TrimSpace(ln)
				if ln != "" {
					info.Items = append(info.Items, ln)
				}
			}
		}
	}

	return info, nil
}
