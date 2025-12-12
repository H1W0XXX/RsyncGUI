//go:build !ui_external
// +build !ui_external

package uiembed

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist/*
var uiRaw embed.FS

func DistFS() (http.FileSystem, error) {
	sub, err := fs.Sub(uiRaw, "web/dist")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
