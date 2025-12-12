import React, { useCallback, useEffect, useState, useRef } from "react";
import { HostInfo, Endpoint, FSEntry } from "../types/api";
import { api } from "../api/client";

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
    const [cwd, setCwd] = useState<string>("");
    const [entries, setEntries] = useState<FSEntry[]>([]);
    const [fsLoading, setFsLoading] = useState(false);
    const [fsError, setFsError] = useState<string | null>(null);

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

        let cancelled = false;

        (async () => {
            try {
                setFsLoading(true);
                setFsError(null);

                const res = await api.fsHome(value.hostName);
                if (cancelled) return;

                setCwd(res.cwd);
                setEntries(res.entries);
                clearSelection();

                onChange({ ...value, path: res.cwd });
            } catch (err: any) {
                if (!cancelled) setFsError(err.message || String(err));
            } finally {
                if (!cancelled) setFsLoading(false);
            }
        })();

        return () => {
            cancelled = true;
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [value.hostName]);

    const refreshDir = useCallback(
        async (path: string) => {
            if (!value.hostName) return;

            try {
                setFsLoading(true);
                setFsError(null);
                const res = await api.fsList(value.hostName, path);

                setCwd(res.cwd);
                setEntries(res.entries);
                clearSelection();
                onChange({ ...value, path: res.cwd });
            } catch (err: any) {
                setFsError(err.message || String(err));
            } finally {
                setFsLoading(false);
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

        refreshDir(joinPathSmart(cwd, entry.name));
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
                alert("请选择 Host，并保证 Path 非空，再拖文件/文件夹进来");
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
                alert(`上传完成（共 ${collected.length} 个文件）`);
            } catch (err: any) {
                console.error(err);
                alert(err.message || String(err));
            } finally {
                setUploading(false);
            }
        },
        [value.hostName, cwd, refreshDir]
    );

    const selSet = new Set(selectedNames ?? []);

    return (
        <div className="card endpoint-card">
            <div className="card-header">
                <div className="card-title">{title}</div>
                {selectedHost && (
                    <div className="card-subtitle">
                        {selectedHost.isLocal
                            ? "This machine"
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
                            {h.isLocal ? "[local]" : h.name}
                            {h.remark ? ` - ${h.remark}` : ""}
                        </option>
                    ))}
                </select>
            </div>

            <div className="field">
                <label>Path</label>
                <input type="text" value={cwd} onChange={handlePathChange} onBlur={handlePathBlur} />
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
                    {uploading ? "Uploading..." : "拖拽文件 / 文件夹到这里，按当前目录结构上传"}
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

                {fsError && <div className="fs-error">{fsError}</div>}

                <div className="fs-list">
                    {canGoParent(cwd) && (
                        <div
                            className={"fs-item dir" + (selSet.has("..") ? " selected" : "")}
                            onClick={(ev) => handleItemClick({ name: "..", isDir: true }, -1, ev)}
                            onDoubleClick={() => handleItemDoubleClick({ name: "..", isDir: true })}
                        >
                            📁 ..
                        </div>
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
