package app

import (
	"sync"
	"time"
)

// Endpoint：一个界面上的端点（左/右之一）
type Endpoint struct {
	HostName string `json:"hostName"`
	Path     string `json:"path"`
}

// RsyncOptions：部分常用选项
type RsyncOptions struct {
	Profile   string   `json:"profile"` // "WAN" / "LAN" / "Custom"
	Archive   bool     `json:"archive"`
	Compress  bool     `json:"compress"`
	Delete    bool     `json:"delete"`
	DryRun    bool     `json:"dryRun"`
	BwLimit   int      `json:"bwlimit"`   // 0 = unlimited
	ExtraArgs []string `json:"extraArgs"` // 额外参数
}

// TransferRequest：前端创建任务时传过来的结构
type TransferRequest struct {
	EndpointA Endpoint     `json:"endpointA"`
	EndpointB Endpoint     `json:"endpointB"`
	Direction string       `json:"direction"` // "A_to_B" or "B_to_A"
	ExecSide  string       `json:"execSide"`  // "auto" / "source" / "dest"
	Options   RsyncOptions `json:"options"`
}

type ExecMode string

const (
	ExecLocal        ExecMode = "local"          // 在本机跑 rsync
	ExecOnSource     ExecMode = "on_source"      // ssh 到源机，在源机跑 rsync
	ExecOnDest       ExecMode = "on_dest"        // ssh 到目标机
	ExecTwoStepLocal ExecMode = "two_step_local" // A→local→B
)

type TransferPlan struct {
	Mode      ExecMode  `json:"mode"`
	Source    Endpoint  `json:"source"`
	Dest      Endpoint  `json:"dest"`
	ExecHost  string    `json:"execHost"` // hostName
	TwoStep   bool      `json:"twoStep"`  // 是否需要 A→local→B
	CreatedAt time.Time `json:"createdAt"`
}

// Job 状态
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobRunning JobStatus = "running"
	JobOK      JobStatus = "success"
	JobFailed  JobStatus = "failed"
	JobCancel  JobStatus = "cancelled"
)

type Job struct {
	ID        string          `json:"id"`
	Request   TransferRequest `json:"request"`
	Plan      TransferPlan    `json:"plan"`
	Status    JobStatus       `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
	StartedAt time.Time       `json:"startedAt"`
	EndedAt   time.Time       `json:"endedAt"`
	LogLines  []string        `json:"logLines"`

	mu       sync.Mutex // 保护 LogLines & Status
	cancelFn func()     // 未来支持取消
}

type PrecheckResult struct {
	SourceReadable bool   `json:"sourceReadable"`
	DestWritable   bool   `json:"destWritable"`
	Message        string `json:"message"`
}
