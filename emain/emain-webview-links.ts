// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { waveEventSubscribeSingle } from "@/app/store/wps";
import { shell } from "electron";
import { fireAndForget } from "../frontend/util/util";
import { callWithOriginalXdgCurrentDesktopAsync } from "./emain-platform";

let externalDomains: string[] = [];

function normalizeDomains(val: any): string[] {
    return Array.isArray(val) ? val.filter((v) => typeof v === "string") : [];
}

export function getExternalDomains(): string[] {
    return externalDomains;
}

export function setExternalDomainsFromSettings(settings: SettingsType) {
    externalDomains = normalizeDomains(settings?.["web:externaldomains"]);
}

export function initExternalDomainsEventSubscription() {
    waveEventSubscribeSingle({
        eventType: "config",
        handler: (event) => {
            try {
                setExternalDomainsFromSettings(event?.data?.fullconfig?.settings);
            } catch (e) {
                console.log("error handling config event for externaldomains", e);
            }
        },
    });
}

export function openExternalUrl(url: string) {
    fireAndForget(() =>
        callWithOriginalXdgCurrentDesktopAsync(() =>
            shell.openExternal(url).catch((err) => {
                console.error(`Failed to open URL ${url}:`, err);
            })
        )
    );
}
