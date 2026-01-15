import React, { useEffect, useMemo, useState } from "react";
import { RsyncOptions } from "../types/api";
import { useTranslation } from "react-i18next";

export type ExecSide = "auto" | "source" | "dest";

interface Props {
    value: RsyncOptions;
    onChange: (val: RsyncOptions) => void;

    // ✅ 远程↔远程时选择第一跳在哪边执行
    execSide: ExecSide;
    onExecSideChange: (v: ExecSide) => void;
}

// 支持简单引号：--exclude='a b' 或 "--exclude=a b"
function splitArgs(text: string): string[] {
    const out: string[] = [];
    let cur = "";
    let quote: "'" | '"' | null = null;

    for (let i = 0; i < text.length; i++) {
        const ch = text[i];

        if (quote) {
            if (ch === quote) {
                quote = null;
            } else {
                cur += ch;
            }
            continue;
        }

        if (ch === "'" || ch === '"') {
            quote = ch;
            continue;
        }

        if (/\s/.test(ch)) {
            if (cur.length > 0) {
                out.push(cur);
                cur = "";
            }
            continue;
        }

        cur += ch;
    }

    if (cur.length > 0) out.push(cur);
    return out;
}

const TransferOptions: React.FC<Props> = ({
                                              value,
                                              onChange,
                                              execSide,
                                              onExecSideChange,
                                          }) => {
    const { t } = useTranslation();
    const set = (patch: Partial<RsyncOptions>) => onChange({ ...value, ...patch });

    // ✅ 关键：不要直接 value.extraArgs.join(" ") 作为受控输入，否则末尾空格会被吞
    const [extraArgsText, setExtraArgsText] = useState<string>("");

    // 当外部 value.extraArgs 改变（比如切换 profile / 切换任务），同步到输入框
    // 用 join("\u0001") 做稳定依赖，避免引用变化导致不必要的同步
    const extraArgsKey = useMemo(
        () => (value.extraArgs ?? []).join("\u0001"),
        [value.extraArgs]
    );

    useEffect(() => {
        setExtraArgsText((value.extraArgs ?? []).join(" "));
    }, [extraArgsKey]);

    const commitExtraArgs = () => {
        const parsed = splitArgs(extraArgsText);
        set({ extraArgs: parsed });
    };

    return (
        <div className="card options-card">
            <div className="card-header">
                <div className="card-title">Rsync Options</div>
                <div className="card-subtitle">Profile &amp; fine-tuning</div>
            </div>

            <div className="field">
                <label>Profile</label>
                <div className="pill-row">
                    {[
                        { k: "LAN", t: t("transfer_options.profile_lan") },
                        { k: "WAN", t: t("transfer_options.profile_wan") }
                    ].map((p) => (
                        <button
                            key={p.k}
                            type="button"
                            className={"pill-btn" + (value.profile === p.k ? " selected" : "")}
                            onClick={() => set({ profile: p.k as any })}
                        >
                            {p.t}
                        </button>
                    ))}
                </div>
            </div>

            <div className="field">
                <label>Remote ↔ Remote exec side (first hop)</label>
                <div className="pill-row">
                    {(
                        [
                            { k: "auto", t: t("transfer_options.exec_side_auto") },
                            { k: "source", t: t("transfer_options.exec_side_source") },
                            { k: "dest", t: t("transfer_options.exec_side_dest") },
                        ] as const
                    ).map((p) => (
                        <button
                            key={p.k}
                            type="button"
                            className={"pill-btn" + (execSide === p.k ? " selected" : "")}
                            onClick={() => onExecSideChange(p.k)}
                        >
                            {p.t}
                        </button>
                    ))}
                </div>
            </div>

            <div className="field field-inline">
                <label>
                    <input
                        type="checkbox"
                        checked={value.archive}
                        onChange={(e) => set({ archive: e.target.checked })}
                    />
                    {t("transfer_options.option_archive")}
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={value.compress}
                        onChange={(e) => set({ compress: e.target.checked })}
                    />
                    {t("transfer_options.option_compress")}
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={value.delete}
                        onChange={(e) => set({ delete: e.target.checked })}
                    />
                    {t("transfer_options.option_delete")}
                </label>
                <label>
                    <input
                        type="checkbox"
                        checked={value.dryRun}
                        onChange={(e) => set({ dryRun: e.target.checked })}
                    />
                    {t("transfer_options.option_dry_run")}
                </label>
            </div>

            <div className="field">
                <label>Bandwidth limit (KB/s, 0 = unlimited)</label>
                <input
                    type="number"
                    min={0}
                    value={value.bwlimit}
                    onChange={(e) => set({ bwlimit: Number(e.target.value || 0) })}
                    placeholder={t("transfer_options.bwlimit_placeholder")}
                />
            </div>

            <div className="field">
                <label>Extra args (space separated)</label>
                <input
                    type="text"
                    placeholder={t("transfer_options.extra_args_placeholder")}
                    value={extraArgsText}
                    onChange={(e) => setExtraArgsText(e.target.value)} // ✅ 不在这里 split，避免吞尾空格
                    onBlur={commitExtraArgs} // ✅ 失焦再解析写回数组
                    onKeyDown={(e) => {
                        if (e.key === "Enter") {
                            commitExtraArgs();
                            (e.target as HTMLInputElement).blur();
                        }
                    }}
                />
            </div>
        </div>
    );
};

export default TransferOptions;
