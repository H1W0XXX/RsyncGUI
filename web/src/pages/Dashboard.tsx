import React, { useEffect, useState } from "react";
import EndpointPanel from "../components/EndpointPanel";
import DirectionSwitch from "../components/DirectionSwitch";
import TransferOptions from "../components/TransferOptions";
import JobsPanel from "../components/JobsPanel";
import { api } from "../api/client";
import { HostInfo, Endpoint, RsyncOptions, TransferRequest, Job } from "../types/api";

const Dashboard: React.FC = () => {
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

    const handleCreateTransfer = async () => {
        if (!endpointA.hostName || !endpointB.hostName) {
            alert("两边 host 都要选上。");
            return;
        }
        if (!endpointA.path || !endpointB.path) {
            alert("两边 path 都要填。");
            return;
        }

        const srcNames = direction === "A_to_B" ? uniqNames(selectedA) : uniqNames(selectedB);
        const namesToSubmit: (string | null)[] = srcNames.length ? srcNames : [null];

        const reqs: TransferRequest[] = namesToSubmit.map((name) => {
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
                alert(`Jobs created (${okIds.length}): ${okIds.join(", ")}`);
            } else {
                alert(
                    `Jobs created: ${okIds.length}/${results.length}\n` +
                    (okIds.length ? `OK: ${okIds.join(", ")}\n` : "") +
                    `Failed:\n${badMsgs.slice(0, 8).join("\n")}${badMsgs.length > 8 ? "\n..." : ""}`
                );
            }

            refreshJobs();
        } catch (err: any) {
            alert(err.message || String(err));
        } finally {
            setCreating(false);
        }
    };

    return (
        <div className="page-root">
            <header className="top-bar">
                <div className="brand">Rsync GUI</div>
                <div className="top-bar-right">
                    <span className="top-hint">私钥只在本机，远程用 ssh/rsync 传输。</span>
                </div>
            </header>

            <main className="layout-main">
                <section className="layout-row main-layout">
                    <div className="endpoint-wrapper">
                        <EndpointPanel
                            title="Endpoint A"
                            hosts={hosts}
                            value={endpointA}
                            onChange={setEndpointA}
                            selectedNames={selectedA}
                            onSelectedChange={setSelectedA}
                        />
                    </div>

                    <div className="center-column">
                        <DirectionSwitch direction={direction} onChange={setDirection} />

                        <button
                            className="primary-btn"
                            onClick={handleCreateTransfer}
                            disabled={creating || loadingHosts}
                        >
                            {creating ? "Creating..." : "Sync"}
                        </button>

                        <TransferOptions
                            value={options}
                            onChange={setOptions}
                            execSide={execSide}
                            onExecSideChange={setExecSide}
                        />
                    </div>

                    <div className="endpoint-wrapper">
                        <EndpointPanel
                            title="Endpoint B"
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
        </div>
    );
};

export default Dashboard;
