//go:build ui_external
// +build ui_external

package uiembed

import (
	"net/http"
	"os"
)

func DistFS() (http.FileSystem, error) {
	dir := os.Getenv("RSYNCGUI_WEB_DIR")
	if dir == "" {
		dir = "web/dist"
	}
	return http.FS(os.DirFS(dir)), nil
}
