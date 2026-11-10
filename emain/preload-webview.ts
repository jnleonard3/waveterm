// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { ipcRenderer } from "electron";

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

console.log("loaded wave preload-webview.ts");
