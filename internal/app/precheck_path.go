package app

import (
	"fmt"
	"strings"
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

	// 1) 是否存在
	_, err := h.Run(fmt.Sprintf(`test -e "%s"`, path))
	info.Exists = err == nil

	if !info.Exists {
		return info, nil
	}

	// 2) 是否目录
	_, err = h.Run(fmt.Sprintf(`test -d "%s"`, path))
	info.IsDir = err == nil

	// 3) 是否可读
	_, err = h.Run(fmt.Sprintf(`test -r "%s"`, path))
	info.Readable = err == nil

	// 4) 是否可写
	_, err = h.Run(fmt.Sprintf(`test -w "%s"`, path))
	info.Writable = err == nil

	// 5) 若是目录，列文件
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
