package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunPrechecks：统一跑源读/目标写权限检查
func (a *App) RunPrechecks(plan *TransferPlan) (*PrecheckResult, error) {
	res := &PrecheckResult{
		SourceReadable: true,
		DestWritable:   true,
		Message:        "",
	}

	if err := a.checkSourceReadable(plan); err != nil {
		res.SourceReadable = false
		res.Message += "source: " + err.Error() + "; "
	}
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
		// ✅ Windows/Linux 都可：不用 shell
		st, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("local source not readable: %v", err)
		}
		if st.IsDir() {
			_, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("local source not readable: %v", err)
			}
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("local source not readable: %v", err)
		}
		_ = f.Close()
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

	if dst.IsLocal {
		// ✅ Windows/Linux 都可：不用 shell
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("local dest not writable: %v", err)
		}
		tmp := filepath.Join(path, fmt.Sprintf(".rsync_gui_test_%d", time.Now().UnixNano()))
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("local dest not writable: %v", err)
		}
		_ = f.Close()
		if err := os.Remove(tmp); err != nil {
			return fmt.Errorf("local dest not writable: %v", err)
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
