// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import * as jotai from "jotai";
import * as React from "react";

export class ProcessViewModel implements ViewModel {
    viewType = "process";
    viewIcon = jotai.atom("play");
    viewName = jotai.atom("Process");
    noPadding = jotai.atom(true);
    constructor(_init: ViewModelInitType) {}
    get viewComponent(): ViewComponent { return ProcessView; }
}

function ProcessView(_props: { model: ProcessViewModel }) {
    const [status, setStatus] = React.useState<ProcessStatusData>(null);
    const [script, setScript] = React.useState("");
    const [cwd, setCwd] = React.useState("");
    const [error, setError] = React.useState("");
    const refresh = React.useCallback(async () => {
        try {
            const next = await RpcApi.ProcessStatusCommand(TabRpcClient);
            setStatus(next);
            setScript((current) => current || next.script || "");
            setCwd((current) => current || next.cwd || "");
            setError("");
        } catch (err) { setError(String(err)); }
    }, []);
    React.useEffect(() => { refresh(); const id = window.setInterval(refresh, 1000); return () => window.clearInterval(id); }, [refresh]);
    const save = async () => {
        try {
            await RpcApi.SetConfigCommand(TabRpcClient, { "app:process": script.trim(), "app:process:cwd": cwd.trim() });
            await refresh();
        } catch (err) { setError(String(err)); }
    };
    const start = async () => { try { await RpcApi.ProcessStartCommand(TabRpcClient); await refresh(); } catch (err) { setError(String(err)); } };
    const stop = async () => { try { await RpcApi.ProcessStopCommand(TabRpcClient); await refresh(); } catch (err) { setError(String(err)); } };
    return <div className="h-full flex flex-col p-4 gap-3 text-sm">
        <div className="text-secondary">Run one local script while Wave is open. It receives a private local Wave launch context and restarts after exit, up to 5 times in 5 minutes.</div>
        <input className="bg-inputbg border border-border rounded px-2 py-1" value={script} placeholder="Command or script" onChange={(e) => setScript(e.target.value)} />
        <input className="bg-inputbg border border-border rounded px-2 py-1" value={cwd} placeholder="Working directory (optional)" onChange={(e) => setCwd(e.target.value)} />
        <div className="flex gap-2"><Button onClick={save}>Save</Button><Button onClick={start} disabled={!status?.script}>Start</Button><Button onClick={stop} disabled={!status?.running}>Stop</Button></div>
        <div className={status?.restartlimithit ? "text-error" : "text-secondary"}>{status?.running ? "Running" : "Stopped"}{status?.restartlimithit ? " — restart limit reached; Start resets it." : ""}{status?.lastexit ? ` — ${status.lastexit}` : ""}</div>
        {error && <div className="text-error">{error}</div>}
        <pre className="flex-1 min-h-0 overflow-auto bg-black/30 border border-border rounded p-3 whitespace-pre-wrap font-mono text-xs">{status?.logs || "No process output yet."}</pre>
    </div>;
}
