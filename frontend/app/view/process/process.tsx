// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { createBlock } from "@/app/store/global";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import * as jotai from "jotai";
import * as React from "react";
import { ProcessRow } from "./processrow";

const PollIntervalMs = 1000;
const ExampleConfig = `"app:processes": [
    { "name": "web", "cmd": "npm run dev", "cwd": "~/project" },
    { "name": "storybook", "cmd": "npm run storybook", "onstartup": false }
]`;

export class ProcessViewModel implements ViewModel {
    viewType = "process";
    viewIcon = jotai.atom("play");
    viewName = jotai.atom("Process");
    noPadding = jotai.atom(true);
    constructor(_init: ViewModelInitType) {}
    get viewComponent(): ViewComponent {
        return ProcessView;
    }
}

function editConfig() {
    createBlock({ meta: { view: "waveconfig", file: "settings.json" } }, false, true);
}

function ProcessView(_props: { model: ProcessViewModel }) {
    const [statuses, setStatuses] = React.useState<ProcessStatusData[]>(null);
    const [error, setError] = React.useState("");

    const refresh = React.useCallback(async () => {
        try {
            setStatuses((await RpcApi.ProcessListCommand(TabRpcClient)) ?? []);
            setError("");
        } catch (err) {
            setError(String(err));
        }
    }, []);

    React.useEffect(() => {
        refresh();
        const timer = window.setInterval(refresh, PollIntervalMs);
        return () => window.clearInterval(timer);
    }, [refresh]);

    const runAction = React.useCallback(
        async (action: (id: string) => Promise<void>, id: string) => {
            try {
                await action(id);
            } catch (err) {
                setError(String(err));
            }
            await refresh();
        },
        [refresh]
    );
    const onStart = React.useCallback(
        (id: string) => runAction((i) => RpcApi.ProcessStartCommand(TabRpcClient, { id: i }), id),
        [runAction]
    );
    const onRestart = React.useCallback(
        (id: string) => runAction((i) => RpcApi.ProcessRestartCommand(TabRpcClient, { id: i }), id),
        [runAction]
    );
    const onStop = React.useCallback(
        (id: string) => runAction((i) => RpcApi.ProcessStopCommand(TabRpcClient, { id: i }), id),
        [runAction]
    );

    const runningCount = statuses?.filter((s) => s.running).length ?? 0;

    return (
        <div className="h-full flex flex-col text-sm">
            <div className="flex items-center gap-2 px-3 py-2 border-b border-border">
                <span className="text-secondary">
                    {statuses?.length ?? 0} {statuses?.length == 1 ? "process" : "processes"}, {runningCount} running
                </span>
                <Button className="ml-auto grey outline cursor-pointer" onClick={editConfig}>
                    Edit config
                </Button>
            </div>
            {error && <div className="text-error px-3 py-2">{error}</div>}
            <div className="flex-1 min-h-0 overflow-auto">
                {statuses != null && statuses.length == 0 && (
                    <div className="p-4 flex flex-col gap-3">
                        <div>No processes configured</div>
                        <div className="text-secondary">Add processes to settings.json:</div>
                        <pre className="bg-black/30 border border-border rounded p-3 font-mono text-xs">
                            {ExampleConfig}
                        </pre>
                        <div>
                            <Button className="cursor-pointer" onClick={editConfig}>
                                Edit config
                            </Button>
                        </div>
                    </div>
                )}
                {statuses?.map((status) => (
                    <ProcessRow
                        key={status.id}
                        status={status}
                        onStart={onStart}
                        onRestart={onRestart}
                        onStop={onStop}
                    />
                ))}
            </div>
        </div>
    );
}
