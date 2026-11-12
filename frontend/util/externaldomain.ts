// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

function extractHost(entry: string): string {
    let host = entry.trim().toLowerCase();
    if (host === "") {
        return "";
    }
    if (host.includes("://")) {
        try {
            host = new URL(host).hostname;
        } catch (e) {
            return "";
        }
    } else {
        host = host.split(/[/?#]/)[0];
    }
    host = host.replace(/^\*\./, "").replace(/^\./, "").replace(/\.$/, "");
    return host;
}

export function isExternalDomainUrl(url: string, domains: string[]): boolean {
    if (!isHttpUrl(url) || domains == null || domains.length === 0) {
        return false;
    }
    const hostname = new URL(url).hostname.toLowerCase().replace(/\.$/, "");
    return domains.some((entry) => {
        if (typeof entry !== "string") {
            return false;
        }
        const domain = extractHost(entry);
        if (domain === "") {
            return false;
        }
        return hostname === domain || hostname.endsWith("." + domain);
    });
}

function isHttpUrl(url: string): boolean {
    if (url == null || url === "") {
        return false;
    }
    try {
        const parsed = new URL(url);
        return parsed.protocol === "http:" || parsed.protocol === "https:";
    } catch (e) {
        return false;
    }
}
