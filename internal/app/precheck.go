package app

import (
	"fmt"
	"strings"
)

// RunPrechecks：统一跑源读/目标写权限检查
func (a *App) RunPrechecks(plan *TransferPlan) (*PrecheckResult, error) {
	res := &PrecheckResult{
		SourceReadable: true,
		DestWritable:   true,
		Message:        "",
	}

	// 检查源
	if err := a.checkSourceReadable(plan); err != nil {
		res.SourceReadable = false
		res.Message += "source: " + err.Error() + "; "
	}

	// 检查目标（以目标目录为单位）
	if err := a.checkDestWritable(plan); err != nil {
		res.DestWritable = false
		res.Message += "dest: " + err.Error() + "; "
	}

	if res.SourceReadable && res.DestWritable {
		res.Message = "ok"
	}
	return res, nil
}

func (a *App) checkSourceReadable(plan *TransferPlan) error {
	src, ok := a.Hosts.Get(plan.Source.HostName)
	if !ok {
		return fmt.Errorf("unknown source host %s", plan.Source.HostName)
	}
	path := plan.Source.Path

	if src.IsLocal {
		// 简单版：直接试一下 ls/dir
		out, err := runLocalShell(fmt.Sprintf("ls %q", path))
		if err != nil {
			return fmt.Errorf("local source not readable: %v (%s)", err, out)
		}
		return nil
	}

	cmd := fmt.Sprintf(`test -r %q && echo OK || echo NO`, path)
	out, err := runSSH(src, cmd)
	if err != nil {
		return fmt.Errorf("remote source read check failed: %v", err)
	}
	if !strings.Contains(out, "OK") {
		return fmt.Errorf("remote source not readable, output: %s", out)
	}
	return nil
}

func (a *App) checkDestWritable(plan *TransferPlan) error {
	dst, ok := a.Hosts.Get(plan.Dest.HostName)
	if !ok {
		return fmt.Errorf("unknown dest host %s", plan.Dest.HostName)
	}
	path := plan.Dest.Path

	// 尝试写入一个临时文件
	if dst.IsLocal {
		cmd := fmt.Sprintf(`mkdir -p %q && echo test > %q/.rsync_gui_test && rm %q/.rsync_gui_test`, path, path, path)
		out, err := runLocalShell(cmd)
		if err != nil {
			return fmt.Errorf("local dest not writable: %v (%s)", err, out)
		}
		return nil
	}

	cmd := fmt.Sprintf(`
DST=%q
mkdir -p "$DST" 2>/dev/null || true
TMP="$DST/.rsync_gui_test_$$"
( touch "$TMP" && rm "$TMP" && echo OK ) || echo NO
`, path)
	out, err := runSSH(dst, cmd)
	if err != nil {
		return fmt.Errorf("remote dest write check failed: %v", err)
	}
	if !strings.Contains(out, "OK") {
		return fmt.Errorf("remote dest not writable, output: %s", out)
	}
	return nil
}
