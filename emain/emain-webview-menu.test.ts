// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
    buildWebviewMenuItems,
    isExternalDomainUrl,
    isExternalOpenableUrl,
    WebviewMenuParams,
} from "./emain-webview-menu";

const actions = (p: WebviewMenuParams) => buildWebviewMenuItems(p).map((i) => i.action);
const PageTail = ["back", "forward", "reload", "separator", "inspect"];

describe("isExternalOpenableUrl", () => {
    it("accepts http and https", () => {
        expect(isExternalOpenableUrl("http://example.com/a")).toBe(true);
        expect(isExternalOpenableUrl("https://example.com/a?b=1")).toBe(true);
    });
    it("rejects other schemes, malformed, and empty", () => {
        expect(isExternalOpenableUrl("javascript:alert(1)")).toBe(false);
        expect(isExternalOpenableUrl("file:///etc/passwd")).toBe(false);
        expect(isExternalOpenableUrl("mailto:a@b.com")).toBe(false);
        expect(isExternalOpenableUrl("not a url")).toBe(false);
        expect(isExternalOpenableUrl("")).toBe(false);
        expect(isExternalOpenableUrl(null)).toBe(false);
    });
});

describe("buildWebviewMenuItems", () => {
    it("empty area shows page and inspect only", () => {
        expect(actions({ linkURL: "", mediaType: "none" })).toEqual(PageTail);
    });
    it("link only", () => {
        expect(actions({ linkURL: "https://a.com", mediaType: "none" })).toEqual([
            "open-link",
            "copy-link",
            "separator",
            ...PageTail,
        ]);
    });
    it("image only", () => {
        expect(actions({ linkURL: "", mediaType: "image", srcURL: "https://a.com/i.png" })).toEqual([
            "save-image",
            "separator",
            ...PageTail,
        ]);
    });
    it("linked image keeps one menu with link, separator, image", () => {
        expect(actions({ linkURL: "https://a.com", mediaType: "image", srcURL: "data:image/png;base64,AA" })).toEqual([
            "open-link",
            "copy-link",
            "separator",
            "save-image",
            "separator",
            ...PageTail,
        ]);
    });
    it("unsafe link scheme yields no link items", () => {
        expect(actions({ linkURL: "javascript:void(0)", mediaType: "none" })).toEqual(PageTail);
        expect(actions({ linkURL: "file:///x", mediaType: "none" })).toEqual(PageTail);
    });
    it("editable field gets Cut/Copy/Paste/Select All enabled from editFlags", () => {
        const items = buildWebviewMenuItems({
            isEditable: true,
            editFlags: { canCut: false, canCopy: true, canPaste: true, canSelectAll: true },
        });
        const edit = items.slice(0, 4);
        expect(edit.map((i) => i.action)).toEqual(["cut", "copy", "paste", "select-all"]);
        expect(edit.map((i) => i.enabled)).toEqual([false, true, true, true]);
        expect(items[4].action).toBe("separator");
    });
    it("editable field with missing editFlags disables everything", () => {
        const items = buildWebviewMenuItems({ isEditable: true });
        expect(items.slice(0, 4).map((i) => i.enabled)).toEqual([false, false, false, false]);
    });
    it("selection on non-editable gives Copy", () => {
        expect(actions({ selectionText: "hello" })).toEqual(["copy", "separator", ...PageTail]);
    });
    it("whitespace-only selection gives no Copy", () => {
        expect(actions({ selectionText: "  \n\t " })).toEqual(PageTail);
    });
    it("editable takes precedence over selection Copy", () => {
        expect(
            actions({ isEditable: true, selectionText: "x", editFlags: {} }).filter((a) => a === "copy")
        ).toHaveLength(1);
    });
    it("Back/Forward enabled from history inputs", () => {
        const enabled = (p: WebviewMenuParams) =>
            buildWebviewMenuItems(p)
                .filter((i) => i.action === "back" || i.action === "forward")
                .map((i) => i.enabled);
        expect(enabled({})).toEqual([false, false]);
        expect(enabled({ canGoBack: true })).toEqual([true, false]);
        expect(enabled({ canGoBack: true, canGoForward: true })).toEqual([true, true]);
    });
    it("full menu has separators only between non-empty sections", () => {
        const result = actions({
            linkURL: "https://a.com",
            mediaType: "image",
            srcURL: "https://a.com/i.png",
            selectionText: "txt",
        });
        expect(result).toEqual([
            "open-link",
            "copy-link",
            "separator",
            "save-image",
            "separator",
            "copy",
            "separator",
            ...PageTail,
        ]);
        expect(result[0]).not.toBe("separator");
        expect(result[result.length - 1]).not.toBe("separator");
    });
});

describe("isExternalDomainUrl", () => {
    const d = ["github.com"];
    it("matches exact host and subdomains", () => {
        expect(isExternalDomainUrl("https://github.com/foo", d)).toBe(true);
        expect(isExternalDomainUrl("https://gist.github.com/x", d)).toBe(true);
        expect(isExternalDomainUrl("http://api.github.com", d)).toBe(true);
    });
    it("rejects look-alike suffixes", () => {
        expect(isExternalDomainUrl("https://notgithub.com", d)).toBe(false);
        expect(isExternalDomainUrl("https://github.com.evil.io", d)).toBe(false);
    });
    it("is case-insensitive", () => {
        expect(isExternalDomainUrl("https://GitHub.COM/x", d)).toBe(true);
        expect(isExternalDomainUrl("https://github.com", ["GitHub.com"])).toBe(true);
    });
    it("accepts wildcard, dot, and URL-form entries", () => {
        expect(isExternalDomainUrl("https://gist.github.com", ["*.github.com"])).toBe(true);
        expect(isExternalDomainUrl("https://github.com", [".github.com"])).toBe(true);
        expect(isExternalDomainUrl("https://github.com/a", ["https://github.com/foo"])).toBe(true);
    });
    it("only considers http/https", () => {
        expect(isExternalDomainUrl("ftp://github.com", d)).toBe(false);
        expect(isExternalDomainUrl("file:///github.com", d)).toBe(false);
        expect(isExternalDomainUrl("garbage", d)).toBe(false);
    });
    it("empty or missing list never matches", () => {
        expect(isExternalDomainUrl("https://github.com", [])).toBe(false);
        expect(isExternalDomainUrl("https://github.com", null)).toBe(false);
        expect(isExternalDomainUrl("https://github.com", ["", "  "])).toBe(false);
    });
});

describe("spellcheck section", () => {
    const edit = { isEditable: true, editFlags: { canPaste: true } };
    it("shows suggestions capped at 5, then add to dictionary, before edit section", () => {
        const items = buildWebviewMenuItems({
            ...edit,
            mediaType: "none",
            misspelledWord: "teh",
            dictionarySuggestions: ["a", "b", "c", "d", "e", "f", "g"],
        });
        expect(items.slice(0, 7).map((i) => i.action)).toEqual([
            "replace-misspelling",
            "replace-misspelling",
            "replace-misspelling",
            "replace-misspelling",
            "replace-misspelling",
            "add-to-dictionary",
            "separator",
        ]);
        expect(items.slice(0, 5).map((i) => i.value)).toEqual(["a", "b", "c", "d", "e"]);
        expect(items[7].action).toBe("cut");
    });
    it("no suggestions shows disabled No Guesses Found", () => {
        const items = buildWebviewMenuItems({ ...edit, misspelledWord: "zzz", dictionarySuggestions: [] });
        expect(items[0]).toMatchObject({ label: "No Guesses Found", enabled: false });
        expect(items[1].action).toBe("add-to-dictionary");
    });
    it("no section when not editable or no misspelling", () => {
        expect(actions({ misspelledWord: "teh", dictionarySuggestions: ["the"] })).toEqual(PageTail);
        expect(actions({ ...edit, misspelledWord: "" })).not.toContain("add-to-dictionary");
    });
    it("separates from link section", () => {
        const a = actions({ ...edit, linkURL: "https://a.com", misspelledWord: "teh", dictionarySuggestions: ["the"] });
        expect(a.slice(0, 5)).toEqual([
            "open-link",
            "copy-link",
            "separator",
            "replace-misspelling",
            "add-to-dictionary",
        ]);
    });
});
