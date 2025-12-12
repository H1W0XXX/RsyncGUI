import {
    HostInfo,
    TransferRequest,
    CreateTransferResponse,
    Job,
    FSListResult,
} from "../types/api";

async function jsonFetch<T>(url: string, init?: RequestInit): Promise<T> {
    const res = await fetch(url, {
        ...init,
        headers: {
            "Content-Type": "application/json",
            ...(init && init.headers)
        }
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(`HTTP ${res.status}: ${text}`);
    }
    return res.json();
}

export const api = {
    async fsHome(hostName: string): Promise<FSListResult> {
        const res = await fetch(`/api/fs/home?host=${encodeURIComponent(hostName)}`);
        if (!res.ok) {
            throw new Error(`fsHome failed: ${res.status} ${await res.text()}`);
        }
        return res.json();
    },

    async fsList(hostName: string, path: string): Promise<FSListResult> {
        const url = `/api/fs/list?host=${encodeURIComponent(
            hostName
        )}&path=${encodeURIComponent(path)}`;
        const res = await fetch(url);
        if (!res.ok) {
            throw new Error(`fsList failed: ${res.status} ${await res.text()}`);
        }
        return res.json();
    },


    async getHosts(): Promise<HostInfo[]> {
        return jsonFetch<HostInfo[]>("/api/hosts");
    },

    async createTransfer(req: TransferRequest): Promise<CreateTransferResponse> {
        return jsonFetch<CreateTransferResponse>("/api/transfers", {
            method: "POST",
            body: JSON.stringify(req)
        });
    },

    async listJobs(): Promise<Job[]> {
        return jsonFetch<Job[]>("/api/jobs");
    },

    async getJob(id: string): Promise<Job> {
        return jsonFetch<Job>(`/api/jobs/${id}`);
    },

    async uploadFile(params: {
        file: File;
        hostName: string;
        path: string;        // dst root
        relPath?: string;    // 相对路径，如 "a.txt" / "dir1/a.txt"
    }) {
        const form = new FormData();
        form.append("file", params.file);
        form.append("hostName", params.hostName);
        form.append("path", params.path);
        if (params.relPath) {
            form.append("relPath", params.relPath);
        }
        const res = await fetch("/api/upload", { method: "POST", body: form });
        if (!res.ok) {
            throw new Error(`upload failed: ${res.status} ${await res.text()}`);
        }
        return res.json();
    }

};

