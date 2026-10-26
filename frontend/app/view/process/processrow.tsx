// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { getApi } from "@/app/store/global";
import { cn } from "@/util/util";
import * as React from "react";
import { ProcessLogTail } from "./processlogtail";

function statusDotClass(status: ProcessStatusData): string {
    if (status.restartlimithit) {
        return "bg-error";
    }
    if (status.running) {
        return "bg-success";
    }
    return "bg-gray-500";
}

function statusText(status: ProcessStatusData): string {
    if (status.running) {
        return `pid ${status.pid}`;
    }
    return status.lastexit ?? "";
}

type ProcessRowProps = {
    status: ProcessStatusData;
    onStart: (id: string) => void;
    onRestart: (id: string) => void;
    onStop: (id: string) => void;
};

export const ProcessRow = React.memo(({ status, onStart, onRestart, onStop }: ProcessRowProps) => {
    const [expanded, setExpanded] = React.useState(false);
    const tooltip = status.cwd ? `${status.cmd}\n(in ${status.cwd})` : status.cmd;
    const text = statusText(status);

    return (
        <div className="border-b border-border px-3 py-2">
            <div className="flex items-center gap-2 min-w-0">
                <button
                    className="cursor-pointer text-secondary hover:text-primary w-4 shrink-0"
                    onClick={() => setExpanded(!expanded)}
                    aria-label={expanded ? "Collapse log" : "Expand log"}
                >
                    <i className={cn("fa-solid text-xs", expanded ? "fa-chevron-down" : "fa-chevron-right")} />
                </button>
                <span className={cn("w-2 h-2 rounded-full shrink-0", statusDotClass(status))} />
                <span className="truncate font-medium min-w-0" title={tooltip}>
                    {status.name}
                </span>
                {text && (
                    <span className="truncate text-secondary text-xs min-w-0" title={text}>
                        {text}
                    </span>
                )}
                {!status.onstartup && (
                    <span className="text-xs text-secondary border border-border rounded px-1 shrink-0">on demand</span>
                )}
                {status.jwt && (
                    <span className="text-xs text-secondary border border-border rounded px-1 shrink-0">
                        wave access
                    </span>
                )}
                {status.restartlimithit && (
                    <span className="text-xs text-error border border-error rounded px-1 shrink-0">
                        restart limit reached
                    </span>
                )}
                <div className="ml-auto flex items-center gap-1 shrink-0">
                    {status.running ? (
                        <>
                            <Button className="grey outline cursor-pointer" onClick={() => onRestart(status.id)}>
                                Restart
                            </Button>
                            <Button className="grey outline cursor-pointer" onClick={() => onStop(status.id)}>
                                Stop
                            </Button>
                        </>
                    ) : (
                        <Button className="cursor-pointer" onClick={() => onStart(status.id)}>
                            Start
                        </Button>
                    )}
                    <button
                        className="cursor-pointer text-secondary hover:text-primary px-1"
                        title="Open log file"
                        aria-label="Open log file"
                        onClick={() => getApi().openNativePath(status.logfile)}
                    >
                        <i className="fa-solid fa-file-lines" />
                    </button>
                </div>
            </div>
            {expanded && (
                <div className="mt-2">
                    <ProcessLogTail id={status.id} />
                </div>
            )}
        </div>
    );
});
ProcessRow.displayName = "ProcessRow";
