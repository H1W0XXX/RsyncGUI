import React, { useCallback, useEffect, useState, useRef } from "react";
import { HostInfo, Endpoint, FSEntry } from "../types/api";
import { api } from "../api/client";
import { useTranslation } from "react-i18next";

interface Props {
    title: string;
    hosts: HostInfo[];
    value: Endpoint;
    onChange: (ep: Endpoint) => void;

    // 多选：当前选中的条目名（相对 cwd 的 name）
    selectedNames?: string[];
    onSelectedChange?: (names: string[]) => void;
}

const EndpointPanel: React.FC<Props> = ({
                                            title,
                                            hosts,
                                            value,
                                            onChange,
                                            selectedNames,
                                            onSelectedChange,
                                        }) => {
    const { t } = useTranslation();
    const [cwd, setCwd] = useState<string>("");
    const [entries, setEntries] = useState<FSEntry[]>([]);
    const [fsLoading, setFsLoading] = useState(false);
    const [fsError, setFsError] = useState<string | null>(null);

    const navSeqRef = useRef(0);
    const abortRef = useRef<AbortController | null>(null);
    const prefetchRef = useRef<Map<string, FSEntry[]>>(new Map());

    const [isDragging, setIsDragging] = useState(false);
    const [uploading, setUploading] = useState(false);

    const selectedHost = hosts.find((h) => h.name === value.hostName);

    function pathStyle(p: string): "win" | "posix" {
        if (!p) return "posix";
        if (/^[A-Za-z]:/.test(p)) return "win"; // C:\  C:/  C:
        if (p.startsWith("\\\\")) return "win"; // UNC
        if (p.startsWith("/")) return "posix"; // /mnt/... etc
        return "posix";
    }

    function trimTrailingSepSmart(p: string): string {
        if (!p) return p;
        const style = pathStyle(p);
        if (style === "win") {
            if (/^[A-Za-z]:[\\/]{1}$/.test(p)) return p; // C:\ 根
            return p.replace(/[\\/]+$/, "");
        }
        return p === "/" ? "/" : p.replace(/\/+$/, "");
    }

    function fmtBytes(n?: number): string {
        const v = n ?? 0;
        if (v < 1024) return `${v} B`;
        const units = ["KB", "MB", "GB", "TB"];
        let x = v / 1024;
        let i = 0;
        while (x >= 1024 && i < units.length - 1) {
            x /= 1024;
            i++;
        }
        return `${x.toFixed(x >= 10 ? 0 : 1)} ${units[i]}`;
    }

    function fmtTime(sec?: number): string {
        if (!sec) return "";
        return new Date(sec * 1000).toLocaleString();
    }

    
    function joinPathSmart(base: string, name: string): string {
        if (!base) return name;
        const b = trimTrailingSepSmart(base);
        const style = pathStyle(b);
        if (style === "win") return b + "\\" + name;
        return (b === "/" ? "" : b) + "/" + name;
    }

    function parentDirSmart(p: string): string {
        if (!p) return p;
        const style = pathStyle(p);
        const s = trimTrailingSepSmart(p);

        if (style === "win") {
            if (/^[A-Za-z]:[\\/]{1}$/.test(s)) return s;
            const idx = Math.max(s.lastIndexOf("\\"), s.lastIndexOf("/"));
            if (idx <= 2) return s.slice(0, 3); // "C:\"
            return s.slice(0, idx);
        }

        if (s === "/") return "/";
        const parts = s.split("/").filter(Boolean);
        if (parts.length <= 1) return "/";
        return "/" + parts.slice(0, -1).join("/");
    }

    function canGoParent(p: string): boolean {
        if (!p) return false;
        const style = pathStyle(p);
        const s = trimTrailingSepSmart(p);
        if (style === "win") return !/^[A-Za-z]:[\\/]{1}$/.test(s);
        return s !== "/";
    }

    const clearSelection = () => {
        onSelectedChange?.([]);
    };

    const storePrefetch = (base: string, children?: Record<string, FSEntry[]>) => {
        if (!base || !children) return;
        for (const [name, ents] of Object.entries(children)) {
            const childPath = joinPathSmart(base, name);
            prefetchRef.current.set(childPath, ents);
        }
    };

    const handleHostChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
        const newHost = e.target.value;
        setCwd("");
        setEntries([]);
        setFsError(null);
        clearSelection();
        onChange({ hostName: newHost, path: "" });
    };

    useEffect(() => {
        if (!value.hostName) {
            setCwd("");
            setEntries([]);
            clearSelection();
            return;
        }

        abortRef.current?.abort();
        navSeqRef.current += 1;
        const seq = navSeqRef.current;
        const controller = new AbortController();
        abortRef.current = controller;
        prefetchRef.current.clear();

        (async () => {
            try {
                setFsLoading(true);
                setFsError(null);

                const res = await api.fsHome(value.hostName, {
                    prefetch: true,
                    maxChildren: 64,
                    signal: controller.signal,
                });
                if (seq !== navSeqRef.current) return;

                setCwd(res.cwd);
                setEntries(res.entries);
                prefetchRef.current.set(res.cwd, res.entries);
                storePrefetch(res.cwd, res.children);
                clearSelection();

                onChange({ ...value, path: res.cwd });
            } catch (err: any) {
                if (err?.name === "AbortError") return;
                if (seq === navSeqRef.current) setFsError(err.message || String(err));
            } finally {
                if (seq === navSeqRef.current) setFsLoading(false);
            }
        })();

        return () => {
            controller.abort();
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [value.hostName]);

    const refreshDir = useCallback(
        async (path: string, opts?: { showLoading?: boolean }) => {
            if (!value.hostName) return;

            abortRef.current?.abort();
            navSeqRef.current += 1;
            const seq = navSeqRef.current;
            const controller = new AbortController();
            abortRef.current = controller;

            const showLoading = opts?.showLoading ?? true;

            try {
                if (showLoading) setFsLoading(true);
                setFsError(null);

                const res = await api.fsList(value.hostName, path, {
                    prefetch: true,
                    maxChildren: 64,
                    signal: controller.signal,
                });
                if (seq !== navSeqRef.current) return;

                setCwd(res.cwd);
                setEntries(res.entries);
                prefetchRef.current.set(res.cwd, res.entries);
                storePrefetch(res.cwd, res.children);
                clearSelection();
                onChange({ ...value, path: res.cwd });
            } catch (err: any) {
                if (err?.name === "AbortError") return;
                if (seq === navSeqRef.current) setFsError(err.message || String(err));
            } finally {
                if (seq === navSeqRef.current && showLoading) setFsLoading(false);
            }
        },
        [value.hostName, value, onChange]
    );

    // ========== 多选：单击/Shift/Ctrl ==========
    const lastClickIndex = useRef<number>(-1);

    const handleItemClick = (
        entry: FSEntry,
        index: number,
        e: React.MouseEvent
    ) => {
        const cur = selectedNames ?? [];
        const name = entry.name;

        // index < 0（比如 ..）不参与 Shift 区间
        const canRange = index >= 0 && lastClickIndex.current >= 0;

        if (e.shiftKey && canRange) {
            const a = Math.min(lastClickIndex.current, index);
            const b = Math.max(lastClickIndex.current, index);
            const namesInRange = entries.slice(a, b + 1).map((x) => x.name);

            const next = e.ctrlKey || e.metaKey
                ? Array.from(new Set([...cur, ...namesInRange]))
                : namesInRange;

            onSelectedChange?.(next);
            lastClickIndex.current = index;
            return;
        }

        if (e.ctrlKey || e.metaKey) {
            const next = cur.includes(name)
                ? cur.filter((x) => x !== name)
                : [...cur, name];
            onSelectedChange?.(next);
            lastClickIndex.current = index >= 0 ? index : -1;
            return;
        }

        onSelectedChange?.([name]);
        lastClickIndex.current = index >= 0 ? index : -1;
    };

    const handleItemDoubleClick = (entry: FSEntry) => {
        if (!entry.isDir) return;

        if (entry.name === "..") {
            refreshDir(parentDirSmart(cwd));
            return;
        }

        const nextPath = joinPathSmart(cwd, entry.name);
        const cached = prefetchRef.current.get(nextPath);
        if (cached) {
            setCwd(nextPath);
            setEntries(cached);
            setFsError(null);
            clearSelection();
            onChange({ ...value, path: nextPath });
            refreshDir(nextPath, { showLoading: false }); // 用新结果覆盖旧时间戳
            return;
        }

        refreshDir(nextPath);
    };

    // ========== 手动输入 path ==========
    const handlePathChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setCwd(e.target.value);
    };

    const handlePathBlur = () => {
        if (value.hostName && cwd) refreshDir(cwd);
    };

    // ========== 拖拽上传 ==========
    async function collectEntriesFromDataTransfer(
        dt: DataTransfer
    ): Promise<{ file: File; relPath: string }[]> {
        const items = Array.from(dt.items);
        const results: { file: File; relPath: string }[] = [];

        const walkEntry = async (entry: any, parentPath: string) => {
            if (!entry) return;

            if (entry.isFile) {
                await new Promise<void>((resolve, reject) => {
                    entry.file(
                        (file: File) => {
                            const relPath = parentPath ? `${parentPath}/${file.name}` : file.name;
                            results.push({ file, relPath });
                            resolve();
                        },
                        reject
                    );
                });
            } else if (entry.isDirectory) {
                const dirReader = entry.createReader();
                const readBatch = (): Promise<void> =>
                    new Promise((resolve, reject) => {
                        dirReader.readEntries(async (ents: any[]) => {
                            if (!ents.length) {
                                resolve();
                                return;
                            }
                            for (const child of ents) {
                                const childParent = parentPath ? `${parentPath}/${entry.name}` : entry.name;
                                await walkEntry(child, childParent);
                            }
                            resolve(await readBatch());
                        }, reject);
                    });
                await readBatch();
            }
        };

        for (const item of items) {
            if (item.kind !== "file") continue;
            const anyItem: any = item as any;
            if (anyItem.webkitGetAsEntry) {
                const entry = anyItem.webkitGetAsEntry();
                if (entry) await walkEntry(entry, "");
            } else {
                const file = item.getAsFile();
                if (file) results.push({ file, relPath: file.name });
            }
        }

        return results;
    }

    const onDragOver = (e: React.DragEvent<HTMLDivElement>) => {
        e.preventDefault();
        setIsDragging(true);
    };
    const onDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
        e.preventDefault();
        setIsDragging(false);
    };

    const onDrop = useCallback(
        async (e: React.DragEvent<HTMLDivElement>) => {
            e.preventDefault();
            setIsDragging(false);

            if (!value.hostName || !cwd) {
                alert(t("endpoint_panel.alert_upload_select_host"));
                return;
            }

            setUploading(true);
            try {
                const collected = await collectEntriesFromDataTransfer(e.dataTransfer);
                if (!collected.length) return;

                for (const { file, relPath } of collected) {
                    await api.uploadFile({
                        file,
                        hostName: value.hostName,
                        path: cwd,
                        relPath,
                    });
                }

                await refreshDir(cwd);
                alert(t("endpoint_panel.alert_upload_complete", { count: collected.length }));
            } catch (err: any) {
                console.error(err);
                alert(err.message || String(err));
            } finally {
                setUploading(false);
            }
        },
        [value.hostName, cwd, refreshDir, t]
    );

    const selSet = new Set(selectedNames ?? []);

    return (
        <div className="card endpoint-card">
            <div className="card-header">
                <div className="card-title">{title}</div>
                {selectedHost && (
                    <div className="card-subtitle">
                        {selectedHost.isLocal
                            ? t("endpoint_panel.host_local")
                            : `${selectedHost.user}@${selectedHost.host}:${selectedHost.port}`}
                    </div>
                )}
            </div>

            <div className="field">
                <label>Host</label>
                <select value={value.hostName} onChange={handleHostChange}>
                    <option value="">Select host...</option>
                    {hosts.map((h) => (
                        <option key={h.name} value={h.name}>
                            {h.isLocal ? `[${t("endpoint_panel.host_local")}]` : h.name}
                            {h.remark ? ` - ${h.remark}` : ""}
                        </option>
                    ))}
                </select>
            </div>

            <div className="field">
                <label>Path</label>
                <input
                    type="text"
                    value={cwd}
                    onChange={handlePathChange}
                    onBlur={handlePathBlur}
                    placeholder={t("endpoint_panel.path_placeholder")}
                />
            </div>

            <div
                className={
                    "drop-zone" +
                    (isDragging ? " drag-over" : "") +
                    (uploading ? " uploading" : "")
                }
                onDragOver={onDragOver}
                onDragLeave={onDragLeave}
                onDrop={onDrop}
            >
                <div className="drop-zone-text">
                    {uploading ? t("endpoint_panel.drop_zone_uploading") : t("endpoint_panel.drop_zone_hint")}
                </div>
            </div>

            <div className="fs-browser">
                <div className="fs-browser-header">
                    <span>Remote Files</span>
                    <button
                        onClick={() => cwd && refreshDir(cwd)}
                        disabled={fsLoading || !value.hostName}
                    >
                        {fsLoading ? "Loading..." : "Refresh"}
                    </button>
                </div>

                {fsError && <div className="fs-error">{t("endpoint_panel.fs_error", { message: fsError })}</div>}

                <div className="fs-list">
                    {canGoParent(cwd) && (
                        <div
                            className={"fs-item dir" + (selSet.has("..") ? " selected" : "")}
                            onClick={(ev) => handleItemClick({ name: "..", isDir: true }, -1, ev)}
                            onDoubleClick={() => handleItemDoubleClick({ name: "..", isDir: true })}
                        >
                            📁 {t("endpoint_panel.parent_dir")}
                        </div>
                    )}

                    {entries.length === 0 && !fsLoading && !fsError && (
                        <div className="fs-empty">{t("endpoint_panel.fs_empty")}</div>
                    )}

                    {entries.map((e, idx) => (
                        <div
                            key={e.name}
                            className={
                                "fs-item " +
                                (e.isDir ? "dir" : "file") +
                                (selSet.has(e.name) ? " selected" : "")
                            }
                            onClick={(ev) => handleItemClick(e, idx, ev)}
                            onDoubleClick={() => handleItemDoubleClick(e)}
                        >
                            <div className="fs-item-left">
                                <span className="fs-icon">{e.isDir ? "📁" : "📄"}</span>
                                <span className="fs-name">{e.name}</span>
                            </div>

                            <div className="fs-item-right">
                                {/* 文件/文件夹都显示最近修改日期 */}
                                <span className="fs-mtime">{fmtTime(e.mtime)}</span>

                                {/* 仅文件显示大小 */}
                                {!e.isDir && <span className="fs-size">{fmtBytes(e.size)}</span>}
                            </div>

                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
};

export default EndpointPanel;
