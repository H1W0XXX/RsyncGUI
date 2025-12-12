package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// GET /api/jobs
func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobs := s.app.JobManager.ListJobs()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jobs)
}

// GET /api/jobs/{id}
func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefix := "/api/jobs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" {
		http.NotFound(w, r)
		return
	}
	job, ok := s.app.JobManager.GetJob(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}
