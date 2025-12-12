package httpapi

import (
	"encoding/json"
	"net/http"
)

// GET /api/fs/home?host=zkyd45
func (s *Server) handleFSHome(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}

	res, err := s.app.HomeDir(host)
	if err != nil {
		http.Error(w, "home dir error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// GET /api/fs/list?host=zkyd45&path=/mnt/data
func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	path := r.URL.Query().Get("path")
	if host == "" || path == "" {
		http.Error(w, "host and path are required", http.StatusBadRequest)
		return
	}

	res, err := s.app.ListDir(host, path)
	if err != nil {
		http.Error(w, "list dir error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
