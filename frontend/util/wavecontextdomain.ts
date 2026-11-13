// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

type WaveContextEntry = {
    host: string;
    port: string;
};

const AuthorityRe = /^(\[[^\]]+\]|[^:\s*]+)(?::(\d{1,5}))?$/;

function parseEntry(raw: string): WaveContextEntry {
    if (typeof raw !== "string") {
        return null;
    }
    let entry = raw.trim().toLowerCase();
    if (entry === "") {
        return null;
    }
    const schemeMatch = entry.match(/^([a-z][a-z0-9+.-]*):\/\//);
    if (schemeMatch != null) {
        if (schemeMatch[1] !== "http" && schemeMatch[1] !== "https") {
            return null;
        }
        entry = entry.slice(schemeMatch[0].length);
    }
    let authority = entry.split(/[/?#]/)[0].replace(/^\*\./, "");
    authority = authority.slice(authority.lastIndexOf("@") + 1);
    const m = authority.match(AuthorityRe);
    if (m == null) {
        return null;
    }
    const host = m[1].replace(/^\./, "").replace(/\.$/, "");
    if (host === "") {
        return null;
    }
    const port = m[2];
    if (port != null) {
        const portNum = parseInt(port, 10);
        if (portNum < 1 || portNum > 65535) {
            return null;
        }
        return { host, port: String(portNum) };
    }
    return { host, port: null };
}

/**
 * Decides whether a page URL is on the `web:wavecontextdomains` allowlist.
 * A bare host matches that host and its subdomains on any port; `host:port` matches
 * that host and port exactly (default ports are implied by the scheme); a full URL is
 * reduced to its host and port. Only http(s) page URLs can match.
 */
export function isWaveContextUrl(url: string, domains: string[]): boolean {
    if (url == null || url === "" || !Array.isArray(domains) || domains.length === 0) {
        return false;
    }
    let parsed: URL;
    try {
        parsed = new URL(url);
    } catch (e) {
        return false;
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        return false;
    }
    const hostname = parsed.hostname.toLowerCase().replace(/\.$/, "");
    const port = parsed.port !== "" ? parsed.port : parsed.protocol === "https:" ? "443" : "80";
    return domains.some((raw) => {
        const entry = parseEntry(raw);
        if (entry == null) {
            return false;
        }
        if (entry.port != null) {
            return hostname === entry.host && port === entry.port;
        }
        return hostname === entry.host || hostname.endsWith("." + entry.host);
    });
}
