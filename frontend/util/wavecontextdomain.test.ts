// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { isWaveContextUrl } from "./wavecontextdomain";

describe("isWaveContextUrl", () => {
    it("bare host matches the host on any port", () => {
        const d = ["example.com"];
        expect(isWaveContextUrl("https://example.com/", d)).toBe(true);
        expect(isWaveContextUrl("http://example.com:8080/x", d)).toBe(true);
        expect(isWaveContextUrl("https://other.com/", d)).toBe(false);
    });
    it("bare host matches subdomains but not lookalikes", () => {
        const d = ["example.com"];
        expect(isWaveContextUrl("https://app.example.com/", d)).toBe(true);
        expect(isWaveContextUrl("https://a.b.example.com/", d)).toBe(true);
        expect(isWaveContextUrl("https://notexample.com/", d)).toBe(false);
        expect(isWaveContextUrl("https://example.com.evil.io/", d)).toBe(false);
    });
    it("is case-insensitive and tolerates wildcard/dot decorations", () => {
        expect(isWaveContextUrl("https://EXAMPLE.com/", ["Example.COM"])).toBe(true);
        expect(isWaveContextUrl("https://a.example.com/", ["*.example.com"])).toBe(true);
        expect(isWaveContextUrl("https://example.com./", ["example.com."])).toBe(true);
    });
    it("host:port matches that port exactly", () => {
        const d = ["localhost:3104"];
        expect(isWaveContextUrl("http://localhost:3104/", d)).toBe(true);
        expect(isWaveContextUrl("http://localhost:3105/", d)).toBe(false);
        expect(isWaveContextUrl("http://localhost/", d)).toBe(false);
    });
    it("host:port does not match subdomains", () => {
        expect(isWaveContextUrl("http://a.localhost:3104/", ["localhost:3104"])).toBe(false);
    });
    it("bare localhost matches every local port", () => {
        expect(isWaveContextUrl("http://localhost:9999/", ["localhost"])).toBe(true);
    });
    it("default ports are implied by the scheme", () => {
        expect(isWaveContextUrl("https://example.com/", ["example.com:443"])).toBe(true);
        expect(isWaveContextUrl("http://example.com/", ["example.com:80"])).toBe(true);
        expect(isWaveContextUrl("http://example.com/", ["example.com:443"])).toBe(false);
    });
    it("full URL entries reduce to host and port", () => {
        expect(isWaveContextUrl("http://localhost:3104/a", ["http://localhost:3104/some/path"])).toBe(true);
        expect(isWaveContextUrl("http://localhost:3105/", ["http://localhost:3104"])).toBe(false);
        expect(isWaveContextUrl("https://app.example.com/", ["https://example.com"])).toBe(true);
    });
    it("ignores userinfo in entries", () => {
        expect(isWaveContextUrl("http://localhost:3104/", ["http://user@localhost:3104"])).toBe(true);
    });
    it("supports ipv6 hosts", () => {
        expect(isWaveContextUrl("http://[::1]:3104/", ["[::1]:3104"])).toBe(true);
        expect(isWaveContextUrl("http://[::1]:3105/", ["[::1]:3104"])).toBe(false);
    });
    it("rejects non-http page URLs", () => {
        const d = ["example.com", "localhost:3104"];
        expect(isWaveContextUrl("file:///etc/passwd", d)).toBe(false);
        expect(isWaveContextUrl("ftp://example.com/", d)).toBe(false);
        expect(isWaveContextUrl("about:blank", d)).toBe(false);
        expect(isWaveContextUrl("javascript:alert(1)", d)).toBe(false);
    });
    it("rejects non-http scheme entries", () => {
        expect(isWaveContextUrl("https://example.com/", ["ftp://example.com"])).toBe(false);
        expect(isWaveContextUrl("https://example.com/", ["file://example.com"])).toBe(false);
    });
    it("ignores malformed entries without matching everything", () => {
        const bad = ["", "   ", ":3104", "*", "*.", ".", "host:notaport", "host:0", "host:70000", "a b", null, 5, {}];
        expect(isWaveContextUrl("http://localhost:3104/", bad as any)).toBe(false);
        expect(isWaveContextUrl("https://example.com/", bad as any)).toBe(false);
    });
    it("handles empty or invalid inputs", () => {
        expect(isWaveContextUrl("https://example.com/", [])).toBe(false);
        expect(isWaveContextUrl("https://example.com/", null)).toBe(false);
        expect(isWaveContextUrl("", ["example.com"])).toBe(false);
        expect(isWaveContextUrl(null, ["example.com"])).toBe(false);
        expect(isWaveContextUrl("not a url", ["example.com"])).toBe(false);
    });
});
