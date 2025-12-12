package app

import (
	"fmt"
	"time"
)

type App struct {
	Hosts      *HostRegistry
	JobManager *JobManager
}

// NewApp 初始化核心 app
func NewApp(hostConfigs []HostConfig) (*App, error) {
	reg, err := NewHostRegistry(hostConfigs)
	if err != nil {
		return nil, err
	}
	return &App{
		Hosts:      reg,
		JobManager: NewJobManager(reg),
	}, nil
}

// PlanTransfer：把 Request 变成 Plan（不做 IO，只是决策）
func (a *App) PlanTransfer(req TransferRequest) (*TransferPlan, error) {
	// 1. 根据 direction 决定 source/dest
	var source, dest Endpoint
	switch req.Direction {
	case "A_to_B":
		source = req.EndpointA
		dest = req.EndpointB
	case "B_to_A":
		source = req.EndpointB
		dest = req.EndpointA
	default:
		return nil, fmt.Errorf("invalid direction: %s", req.Direction)
	}

	srcHost, ok := a.Hosts.Get(source.HostName)
	if !ok {
		return nil, fmt.Errorf("unknown source host: %s", source.HostName)
	}
	dstHost, ok := a.Hosts.Get(dest.HostName)
	if !ok {
		return nil, fmt.Errorf("unknown dest host: %s", dest.HostName)
	}

	plan := &TransferPlan{
		Source:    source,
		Dest:      dest,
		CreatedAt: time.Now(),
	}

	// 2. 决定执行模式
	if srcHost.IsLocal || dstHost.IsLocal {
		// 一端是本机 → 在本机跑 rsync
		plan.Mode = ExecLocal
		plan.ExecHost = "local"
		plan.TwoStep = false
		return plan, nil
	}

	// 两端都是远程
	switch req.ExecSide {
	case "source":
		plan.Mode = ExecOnSource
		plan.ExecHost = srcHost.Config.Name
		plan.TwoStep = false
	case "dest":
		plan.Mode = ExecOnDest
		plan.ExecHost = dstHost.Config.Name
		plan.TwoStep = false
	default: // "auto"
		// 简单版本：优先源机执行，以后可以加拓扑判断
		plan.Mode = ExecOnSource
		plan.ExecHost = srcHost.Config.Name
		plan.TwoStep = false
	}

	return plan, nil
}
