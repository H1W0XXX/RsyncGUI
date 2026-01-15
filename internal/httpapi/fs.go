package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// GET /api/fs/home?host=zkyd45
func (s *Server) handleFSHome(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}

	prefetch := parseBoolish(r.URL.Query().Get("prefetch"))
	maxChildren := parseIntDefault(r.URL.Query().Get("maxChildren"), 0)

	res, err := s.app.HomeDirEx(host, prefetch, maxChildren)
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

	prefetch := parseBoolish(r.URL.Query().Get("prefetch"))
	maxChildren := parseIntDefault(r.URL.Query().Get("maxChildren"), 0)

	res, err := s.app.ListDirEx(host, path, prefetch, maxChildren)
	if err != nil {
		http.Error(w, "list dir error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func parseBoolish(v string) bool {
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseIntDefault(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
