import React from "react";
import { Job } from "../types/api";

interface Props {
    jobs: Job[];
    onRefresh: () => void;
}

const statusColor: Record<Job["status"], string> = {
    pending: "#aaa",
    running: "#ffd166",
    success: "#06d6a0",
    failed: "#ef476f",
    cancelled: "#888"
};

const JobsPanel: React.FC<Props> = ({ jobs, onRefresh }) => {
    return (
        <div className="card jobs-card">
            <div className="card-header">
                <div className="card-title">Jobs</div>
                <button className="small-btn" onClick={onRefresh}>
                    Refresh
                </button>
            </div>
            {jobs.length === 0 && (
                <div className="empty-hint">还没有任务，先配置两端然后点 Sync。</div>
            )}
            <div className="jobs-list">
                {jobs
                    .slice()
                    .reverse()
                    .map(job => (
                        <div className="job-item" key={job.id}>
                            <div className="job-header">
                                <span className="job-id">{job.id.slice(0, 8)}</span>
                                <span
                                    className="job-status-dot"
                                    style={{ backgroundColor: statusColor[job.status] }}
                                ></span>
                                <span className="job-status-text">{job.status}</span>
                                <span className="job-mode-tag">
                  {job.plan.mode} @ {job.plan.execHost}
                </span>
                            </div>
                            <div className="job-paths">
                                <div className="job-path-line">
                                    <strong>Src:</strong>{" "}
                                    {job.plan.source.hostName}:{job.plan.source.path}
                                </div>
                                <div className="job-path-line">
                                    <strong>Dst:</strong>{" "}
                                    {job.plan.dest.hostName}:{job.plan.dest.path}
                                </div>
                            </div>
                            {job.logLines && job.logLines.length > 0 && (
                                <details className="job-logs">
                                    <summary>Logs</summary>
                                    <pre>
                    {job.logLines.slice(-50).join("\n")}
                                        {job.logLines.length > 50 ? "\n..." : ""}
                  </pre>
                                </details>
                            )}
                        </div>
                    ))}
            </div>
        </div>
    );
};

export default JobsPanel;
