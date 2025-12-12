export interface HostInfo {
    name: string;
    remark: string;
    host: string;
    port: number;
    user: string;
    isLocal: boolean;
}

export interface Endpoint {
    hostName: string;
    path: string;
}

export interface RsyncOptions {
    profile: "WAN" | "LAN" | "Custom";
    archive: boolean;
    compress: boolean;
    delete: boolean;
    dryRun: boolean;
    bwlimit: number;
    extraArgs: string[];
}

export type ExecSide = "auto" | "source" | "dest";

export interface TransferRequest {
    endpointA: Endpoint;
    endpointB: Endpoint;
    direction: "A_to_B" | "B_to_A";
    execSide?: ExecSide;

    options: RsyncOptions;
}

export type ExecMode =
    | "local"
    | "on_source"
    | "on_dest"
    | "two_step_local";

export interface TransferPlan {
    mode: ExecMode;
    source: Endpoint;
    dest: Endpoint;
    execHost: string;
    twoStep: boolean;
    createdAt: string;
}

export interface PrecheckResult {
    sourceReadable: boolean;
    destWritable: boolean;
    message: string;
}

export type JobStatus =
    | "pending"
    | "running"
    | "success"
    | "failed"
    | "cancelled";

export interface Job {
    id: string;
    request: TransferRequest;
    plan: TransferPlan;
    status: JobStatus;
    createdAt: string;
    startedAt: string;
    endedAt: string;
    logLines: string[];
}

export interface CreateTransferResponse {
    jobId: string;
    plan: TransferPlan;
    precheck: PrecheckResult;
}


export interface FSEntry {
    name: string;
    isDir: boolean;

    // 最近修改时间（unix 秒）
    mtime?: number;

    // 文件大小（字节），目录可为 0 或不传
    size?: number;
}

export interface FSListResult {
    cwd: string;
    entries: FSEntry[];
}