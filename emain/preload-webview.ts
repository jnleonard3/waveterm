// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { contextBridge, ipcRenderer } from "electron";

document.addEventListener("mouseup", (event) => {
    // Mouse button 3 = back, button 4 = forward
    if (!event.isTrusted) {
        return;
    }
    if (event.button === 3 || event.button === 4) {
        event.preventDefault();
        ipcRenderer.send("webview-mouse-navigate", event.button === 3 ? "back" : "forward");
    }
});

// The webview runs with context isolation on (the <webview> sets no webpreferences override), so the page's
// main world only sees what contextBridge exposes. The main process decides per call whether this page may
// get the context; this stub carries no authority of its own.
contextBridge.exposeInMainWorld("waveContext", {
    get: (): Promise<WaveContext | null> => ipcRenderer.invoke("webview-get-wave-context"),
});

console.log("loaded wave preload-webview.ts");
