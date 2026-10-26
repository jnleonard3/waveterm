// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { Button } from "@/app/element/button";
import { RpcApi } from "@/app/store/wshclientapi";
import { TabRpcClient } from "@/app/store/wshrpcutil";
import * as React from "react";

const LogLimit = 400;
const PollIntervalMs = 1000;
const BottomSlackPx = 4;

function atBottom(el: HTMLElement): boolean {
    return el.scrollHeight - el.scrollTop - el.clientHeight < BottomSlackPx;
}

// Lines are appended to the DOM directly (instead of via React state) so that the
// scroll position can be corrected by the exact pixel height of lines trimmed off the top.
export const ProcessLogTail = React.memo(({ id }: { id: string }) => {
    const logRef = React.useRef<HTMLDivElement>(null);
    const seqRef = React.useRef(0);
    const pinnedRef = React.useRef(true);
    const [pinned, setPinned] = React.useState(true);

    const appendLines = React.useCallback((seq: number, lines: string[]) => {
        const log = logRef.current;
        if (log == null) {
            return;
        }
        if (seq < seqRef.current) {
            log.replaceChildren();
        }
        seqRef.current = seq;
        if (lines.length == 0) {
            return;
        }
        log.append(
            ...lines.map((line) => {
                const div = document.createElement("div");
                div.textContent = line;
                return div;
            })
        );
        let removed = 0;
        while (log.childElementCount > LogLimit) {
            removed += (log.firstElementChild as HTMLElement).offsetHeight;
            log.firstElementChild.remove();
        }
        if (pinnedRef.current) {
            log.scrollTop = log.scrollHeight;
        } else if (removed > 0) {
            log.scrollTop -= removed;
        }
    }, []);

    React.useEffect(() => {
        let cancelled = false;
        let inFlight = false;
        const poll = async () => {
            if (inFlight) {
                return;
            }
            inFlight = true;
            try {
                const res = await RpcApi.ProcessLogsCommand(TabRpcClient, { id, sinceseq: seqRef.current });
                if (!cancelled) {
                    appendLines(res.seq, res.lines ?? []);
                }
            } catch (_) {
                // transient failures are retried on the next poll
            } finally {
                inFlight = false;
            }
        };
        poll();
        const timer = window.setInterval(poll, PollIntervalMs);
        return () => {
            cancelled = true;
            window.clearInterval(timer);
        };
    }, [id, appendLines]);

    const onScroll = React.useCallback(() => {
        const isPinned = atBottom(logRef.current);
        pinnedRef.current = isPinned;
        setPinned(isPinned);
    }, []);

    const jumpToLatest = React.useCallback(() => {
        pinnedRef.current = true;
        setPinned(true);
        logRef.current.scrollTop = logRef.current.scrollHeight;
    }, []);

    return (
        <div className="relative">
            <div
                ref={logRef}
                onScroll={onScroll}
                className="max-h-[260px] min-h-[60px] overflow-auto bg-black/30 border border-border rounded p-2 font-mono text-xs whitespace-pre-wrap break-words"
            />
            {!pinned && (
                <Button className="absolute bottom-2 right-4 cursor-pointer" onClick={jumpToLatest}>
                    Jump to latest
                </Button>
            )}
        </div>
    );
});
ProcessLogTail.displayName = "ProcessLogTail";
