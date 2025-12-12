package httpapi

import (
	"encoding/json"
	"net/http"
)

type hostDTO struct {
	Name    string `json:"name"`
	Remark  string `json:"remark"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	User    string `json:"user"`
	IsLocal bool   `json:"isLocal"`
}

// GET /api/hosts
func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hosts := s.app.Hosts.All()
	out := make([]hostDTO, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, hostDTO{
			Name:    h.Config.Name,
			Remark:  h.Config.Remark,
			Host:    h.Config.Host,
			Port:    h.Config.Port,
			User:    h.Config.User,
			IsLocal: h.IsLocal,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
