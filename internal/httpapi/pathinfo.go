package httpapi

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handlePathInfo(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	path := r.URL.Query().Get("path")
	if host == "" || path == "" {
		http.Error(w, "missing host or path", 400)
		return
	}

	info, err := s.app.CheckPath(host, path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
