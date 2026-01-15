import React from "react";
import { Job } from "../types/api";
import { useTranslation } from "react-i18next";

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
    const { t } = useTranslation();

    const fmtExecHost = (execHost: string) => {
        if (execHost === "local") return t("endpoint_panel.host_local");
        return execHost;
    };

    const fmtMode = (mode: Job["plan"]["mode"]) => {
        switch (mode) {
            case "local":
                return t("jobs_panel.mode_local");
            case "on_source":
                return t("jobs_panel.mode_on_source");
            case "on_dest":
                return t("jobs_panel.mode_on_dest");
            case "two_step_local":
                return t("jobs_panel.mode_two_step_local");
            default:
                return mode;
        }
    };

    return (
        <div className="card jobs-card">
            <div className="card-header">
                <div className="card-title">Jobs</div>
                <button className="small-btn" onClick={onRefresh}>
                    Refresh
                </button>
            </div>
            {jobs.length === 0 && (
                <div className="empty-hint">{t("jobs_panel.empty_hint")}</div>
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
                                    {t("jobs_panel.exec_on")}: {fmtExecHost(job.plan.execHost)} ·{" "}
                                    {t("jobs_panel.mode")}: {fmtMode(job.plan.mode)}
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
