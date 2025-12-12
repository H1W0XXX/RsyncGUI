package httpapi

import (
	"encoding/json"
	"net/http"

	"rsyncgui/internal/app" // ← 这里按你自己的 module 路径改，比如 "github.com/xxx/rsyncgui/internal/app"
)

// POST /api/transfers
// Body: app.TransferRequest（前端传的 JSON）
func (s *Server) handleTransfers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req app.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 1. 生成执行计划（决定 source/dest + ExecLocal / OnSource / OnDest）
	plan, err := s.app.PlanTransfer(req)
	if err != nil {
		http.Error(w, "plan error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 2. 做一轮预检查：源是否可读、目标是否可写
	precheck, err := s.app.RunPrechecks(plan)
	if err != nil {
		http.Error(w, "precheck error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !precheck.SourceReadable || !precheck.DestWritable {
		// 预检查不过，直接返回给前端，不创建任务
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Error    string              `json:"error"`
			Precheck *app.PrecheckResult `json:"precheck"`
		}{
			Error:    "precheck failed",
			Precheck: precheck,
		})
		return
	}

	// 3. 创建 Job 并异步启动
	job := s.app.JobManager.NewJob(req, plan)
	s.app.JobManager.StartJob(job)

	// 4. 返回 Job 信息 + 预检查结果
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		JobID    string              `json:"jobId"`
		Plan     *app.TransferPlan   `json:"plan"`
		Precheck *app.PrecheckResult `json:"precheck"`
	}{
		JobID:    job.ID,
		Plan:     plan,
		Precheck: precheck,
	})
}
