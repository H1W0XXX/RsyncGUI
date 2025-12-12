package httpapi

import (
	"net/http"
	"strings"
	"time"

	"rsyncgui/internal/app"
	"rsyncgui/internal/uiembed"
)

type Server struct {
	app *app.App
	mux *http.ServeMux
}

func NewServer(core *app.App) http.Handler {
	s := &Server{
		app: core,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s.mux
}

func (s *Server) routes() {
	// API
	s.mux.HandleFunc("/api/hosts", s.handleHosts)
	s.mux.HandleFunc("/api/transfers", s.handleTransfers)
	s.mux.HandleFunc("/api/jobs", s.handleJobs)
	s.mux.HandleFunc("/api/jobs/", s.handleJobDetail) // /api/jobs/{id}
	s.mux.HandleFunc("/api/upload", s.handleUpload)
	s.mux.HandleFunc("/api/pathinfo", s.handlePathInfo)

	s.mux.HandleFunc("/api/fs/home", s.handleFSHome)
	s.mux.HandleFunc("/api/fs/list", s.handleFSList)

	// 前端静态文件（embed 或外置，由 uiembed 的 build tag 决定）
	distFS, err := uiembed.DistFS()
	if err != nil {
		// 兜底：UI 没初始化成功就返回 500
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "ui fs init failed", http.StatusInternalServerError)
		})
		return
	}

	// Vite 默认静态资源路径 /assets/*
	s.mux.Handle("/assets/", http.FileServer(distFS))

	// 其他路径：优先尝试当作静态文件，不存在则回退 index.html（SPA）
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 避免 API 被前端吃掉
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 先尝试直接打开对应文件（比如 favicon.ico、robots.txt）
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := distFS.Open(path); err == nil {
				defer f.Close()
				http.ServeContent(w, r, path, time.Now(), f)
				return
			}
		}

		// fallback 到 index.html
		index, err := distFS.Open("index.html")
		if err != nil {
			http.Error(w, "index missing", http.StatusInternalServerError)
			return
		}
		defer index.Close()
		http.ServeContent(w, r, "index.html", time.Now(), index)
	})
}
