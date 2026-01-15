import React, { useEffect, useState } from "react";
import EndpointPanel from "../components/EndpointPanel";
import DirectionSwitch from "../components/DirectionSwitch";
import TransferOptions from "../components/TransferOptions";
import JobsPanel from "../components/JobsPanel";
import { api } from "../api/client";
import { HostInfo, Endpoint, RsyncOptions, TransferRequest, Job } from "../types/api";
import { useTheme } from "../context/ThemeContext";
import { useTranslation } from "react-i18next";

const Dashboard: React.FC = () => {
    const { theme, toggleTheme } = useTheme();
    const { t, i18n } = useTranslation();
    const [selectedA, setSelectedA] = useState<string[]>([]);
    const [selectedB, setSelectedB] = useState<string[]>([]);

    const [hosts, setHosts] = useState<HostInfo[]>([]);
    const [loadingHosts, setLoadingHosts] = useState(true);
    const [direction, setDirection] = useState<"A_to_B" | "B_to_A">("A_to_B");

    const [endpointA, setEndpointA] = useState<Endpoint>({ hostName: "local", path: "" });
    const [endpointB, setEndpointB] = useState<Endpoint>({ hostName: "", path: "" });

    const [execSide, setExecSide] = useState<"auto" | "source" | "dest">("auto");

    const [options, setOptions] = useState<RsyncOptions>({
        profile: "LAN",
        archive: true,
        compress: false,
        delete: false,
        dryRun: false,
        bwlimit: 0,
        extraArgs: ["--progress"],
    });

    const [jobs, setJobs] = useState<Job[]>([]);
    const [creating, setCreating] = useState(false);
    const [previewData, setPreviewData] = useState<string | null>(null);
    const [previewing, setPreviewing] = useState(false);

    // --- path utils（和 EndpointPanel 一致的风格，避免 Windows/WSL 拼错） ---
    function pathStyle(p: string): "win" | "posix" {
        if (!p) return "posix";
        if (/^[A-Za-z]:/.test(p)) return "win";
        if (p.startsWith("\\\\")) return "win";
        if (p.startsWith("/")) return "posix";
        return "posix";
    }
    function trimTrailingSepSmart(p: string): string {
        if (!p) return p;
        const style = pathStyle(p);
        if (style === "win") {
            if (/^[A-Za-z]:[\\/]{1}$/.test(p)) return p;
            return p.replace(/[\\/]+$/, "");
        }
        return p === "/" ? "/" : p.replace(/\/+$/, "");
    }
    function joinPathSmart(base: string, name: string): string {
        if (!base) return name;
        const b = trimTrailingSepSmart(base);
        const style = pathStyle(b);
        if (style === "win") return b + "\\" + name;
        return (b === "/" ? "" : b) + "/" + name;
    }
    const joinPath = (dir: string, name: string | null): string => {
        if (!name || name === "..") return dir;
        return joinPathSmart(dir, name);
    };

    const uniqNames = (arr: string[]) =>
        Array.from(new Set(arr)).filter((n) => n && n !== "..");

    useEffect(() => {
        (async () => {
            try {
                const hs = await api.getHosts();
                setHosts(hs);
                if (!endpointA.hostName) {
                    const local = hs.find((h) => h.isLocal);
                    if (local) setEndpointA((ep) => ({ ...ep, hostName: local.name }));
                }
            } catch (err) {
                console.error(err);
            } finally {
                setLoadingHosts(false);
            }
        })();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const refreshJobs = async () => {
        try {
            const js = await api.listJobs();
            setJobs(js);
        } catch (err) {
            console.error(err);
        }
    };

    useEffect(() => {
        const id = setInterval(refreshJobs, 3000);
        return () => clearInterval(id);
    }, []);

    const buildRequests = () => {
        const srcNames = direction === "A_to_B" ? uniqNames(selectedA) : uniqNames(selectedB);
        const namesToSubmit: (string | null)[] = srcNames.length ? srcNames : [null];

        return namesToSubmit.map((name) => {
            let finalEndpointA: Endpoint = endpointA;
            let finalEndpointB: Endpoint = endpointB;

            if (direction === "A_to_B") {
                finalEndpointA = { ...endpointA, path: joinPath(endpointA.path, name) };
                finalEndpointB = { ...endpointB, path: endpointB.path };
            } else {
                finalEndpointA = { ...endpointA, path: endpointA.path };
                finalEndpointB = { ...endpointB, path: joinPath(endpointB.path, name) };
            }

            return {
                endpointA: finalEndpointA,
                endpointB: finalEndpointB,
                direction,
                execSide,
                options,
            };
        });
    };

    const handleCreateTransfer = async () => {
        if (!endpointA.hostName || !endpointB.hostName) {
            alert(t("dashboard.alert_host_missing"));
            return;
        }
        if (!endpointA.path || !endpointB.path) {
            alert(t("dashboard.alert_path_missing"));
            return;
        }

        const reqs = buildRequests();

        setCreating(true);
        try {
            // ✅ 一次性并发全提交（不串行等待每个 job）
            const results = await Promise.allSettled(reqs.map((r) => api.createTransfer(r)));

            const okIds: string[] = [];
            const badMsgs: string[] = [];

            for (const r of results) {
                if (r.status === "fulfilled") {
                    okIds.push(r.value.jobId);
                } else {
                    badMsgs.push(r.reason?.message || String(r.reason));
                }
            }

            if (badMsgs.length === 0) {
                alert(t("dashboard.alert_jobs_created", { count: okIds.length, ids: okIds.join(", ") }));
            } else {
                alert(
                    t("dashboard.alert_jobs_failed", {
                        ok: okIds.length,
                        total: results.length,
                        okIds: okIds.length ? okIds.join(", ") : "",
                        failedMsgs: badMsgs.slice(0, 8).join("\n") + (badMsgs.length > 8 ? "\n..." : "")
                    })
                );
            }

            refreshJobs();
        } catch (err: any) {
            alert(err.message || String(err));
        } finally {
            setCreating(false);
        }
    };

    const handlePreview = async () => {
        if (!endpointA.hostName || !endpointB.hostName) {
            alert(t("dashboard.alert_host_missing"));
            return;
        }
        if (!endpointA.path || !endpointB.path) {
            alert(t("dashboard.alert_path_missing"));
            return;
        }

        const reqs = buildRequests();
        setPreviewing(true);
        try {
            const results = await Promise.all(reqs.map(r => api.previewTransfer(r)));
            const cmds = results.map(r => r.command).join("\n\n");
            setPreviewData(cmds);
        } catch (err: any) {
            alert(err.message || String(err));
        } finally {
            setPreviewing(false);
        }
    };

    const toggleLanguage = () => {
        const nextLang = i18n.language === "en" ? "zh" : "en";
        i18n.changeLanguage(nextLang);
    };

    return (
        <div className="page-root">
            <header className="top-bar">
                <div className="brand">{t("top_bar.title")}</div>
                <div className="top-bar-right" style={{ display: "flex", alignItems: "center", gap: "16px" }}>
                    <button
                        onClick={toggleLanguage}
                        style={{
                            background: "transparent",
                            border: "1px solid var(--border-subtle)",
                            color: "var(--text-main)",
                            padding: "4px 8px",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "12px",
                        }}
                    >
                        {i18n.language === "en" ? "🇺🇸 English" : "🇨🇳 中文"}
                    </button>
                    <button
                        onClick={toggleTheme}
                        style={{
                            background: "transparent",
                            border: "1px solid var(--border-subtle)",
                            color: "var(--text-main)",
                            padding: "4px 8px",
                            borderRadius: "4px",
                            cursor: "pointer",
                            fontSize: "12px",
                        }}
                    >
                        {theme === "dark" ? t("top_bar.theme_light") : t("top_bar.theme_dark")}
                    </button>
                    <span className="top-hint">{t("top_bar.hint")}</span>
                </div>
            </header>

            <main className="layout-main">
                <section className="layout-row main-layout">
                    <div className="endpoint-wrapper">
                        <EndpointPanel
                            title={t("dashboard.endpoint_a")}
                            hosts={hosts}
                            value={endpointA}
                            onChange={setEndpointA}
                            selectedNames={selectedA}
                            onSelectedChange={setSelectedA}
                        />
                    </div>

                    <div className="center-column">
                        <DirectionSwitch direction={direction} onChange={setDirection} />

                        <div style={{ display: "flex", gap: "8px" }}>
                            <button
                                className="primary-btn"
                                onClick={handleCreateTransfer}
                                disabled={creating || loadingHosts}
                                style={{ flex: 1 }}
                            >
                                {creating ? t("dashboard.creating") : t("dashboard.sync_btn")}
                            </button>
                            <button
                                className="primary-btn"
                                onClick={handlePreview}
                                disabled={previewing || loadingHosts}
                                style={{
                                    flex: 1,
                                    backgroundColor: "var(--bg-elevated)",
                                    border: "1px solid var(--border-subtle)",
                                    color: "var(--text-main)"
                                }}
                            >
                                {previewing ? "..." : t("dashboard.preview_btn")}
                            </button>
                        </div>

                        <TransferOptions
                            value={options}
                            onChange={setOptions}
                            execSide={execSide}
                            onExecSideChange={setExecSide}
                        />
                    </div>

                    <div className="endpoint-wrapper">
                        <EndpointPanel
                            title={t("dashboard.endpoint_b")}
                            hosts={hosts}
                            value={endpointB}
                            onChange={setEndpointB}
                            selectedNames={selectedB}
                            onSelectedChange={setSelectedB}
                        />
                    </div>
                </section>

                <section className="layout-row">
                    <JobsPanel jobs={jobs} onRefresh={refreshJobs} />
                </section>
            </main>

            {previewData !== null && (
                <div style={{
                    position: "fixed",
                    top: 0,
                    left: 0,
                    right: 0,
                    bottom: 0,
                    background: "rgba(0,0,0,0.6)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    zIndex: 100,
                    backdropFilter: "blur(4px)"
                }}>
                    <div style={{
                        background: "var(--bg)",
                        border: "1px solid var(--border-subtle)",
                        borderRadius: "12px",
                        width: "800px",
                        maxWidth: "90vw",
                        maxHeight: "80vh",
                        display: "flex",
                        flexDirection: "column",
                        boxShadow: "0 20px 40px rgba(0,0,0,0.5)"
                    }}>
                        <div style={{
                            padding: "16px 20px",
                            borderBottom: "1px solid var(--border-subtle)",
                            display: "flex",
                            justifyContent: "space-between",
                            alignItems: "center"
                        }}>
                            <h3 style={{ margin: 0, fontSize: "16px" }}>{t("dashboard.preview_title")}</h3>
                            <button
                                onClick={() => setPreviewData(null)}
                                style={{
                                    background: "transparent",
                                    border: "none",
                                    color: "var(--text-muted)",
                                    cursor: "pointer",
                                    fontSize: "20px"
                                }}
                            >
                                ×
                            </button>
                        </div>
                        <div style={{
                            padding: "20px",
                            overflowY: "auto",
                            flex: 1
                        }}>
                            <pre style={{
                                margin: 0,
                                whiteSpace: "pre-wrap",
                                fontFamily: "monospace",
                                fontSize: "13px",
                                color: "var(--text-main)",
                                background: "var(--bg-elevated)",
                                padding: "12px",
                                borderRadius: "8px",
                                border: "1px solid var(--border-subtle)"
                            }}>
                                {previewData}
                            </pre>
                        </div>
                        <div style={{
                            padding: "16px 20px",
                            borderTop: "1px solid var(--border-subtle)",
                            textAlign: "right"
                        }}>
                            <button
                                onClick={() => setPreviewData(null)}
                                style={{
                                    padding: "8px 16px",
                                    borderRadius: "6px",
                                    border: "1px solid var(--border-subtle)",
                                    background: "var(--bg-elevated)",
                                    color: "var(--text-main)",
                                    cursor: "pointer"
                                }}
                            >
                                {t("dashboard.preview_close")}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default Dashboard;
