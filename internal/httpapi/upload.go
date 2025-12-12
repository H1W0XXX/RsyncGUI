package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// POST /api/upload
// multipart/form-data:
//   - file: 文件
//   - hostName: string
//   - path: 目标目录
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 限制下大小，比如 100MB，后面可以做成配置
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	hostName := r.FormValue("hostName")
	dstPath := r.FormValue("path")
	relPath := r.FormValue("relPath")
	
	if hostName == "" || dstPath == "" {
		http.Error(w, "hostName and path are required", http.StatusBadRequest)
		return
	}

	// 先落到本机临时目录
	tmpDir := "data/uploads"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		http.Error(w, "create upload dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpFile, err := os.CreateTemp(tmpDir, "upload-*")
	if err != nil {
		http.Error(w, "create temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		_ = os.Remove(tmpPath)
		http.Error(w, "write temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 调用 app 层，把本地临时文件推到指定 host:/path
	baseName := filepath.Base(header.Filename)
	if err := s.app.UploadLocalFile(hostName, dstPath, relPath, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		http.Error(w, "upload to host failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		OK       bool   `json:"ok"`
		HostName string `json:"hostName"`
		Path     string `json:"path"`
		FileName string `json:"fileName"`
	}{
		OK:       true,
		HostName: hostName,
		Path:     dstPath,
		FileName: baseName,
	})
}
