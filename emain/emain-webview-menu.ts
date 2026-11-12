// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import { isExternalDomainUrl } from "@/util/externaldomain";

export { isExternalDomainUrl };

export type WebviewEditFlags = {
    canCut?: boolean;
    canCopy?: boolean;
    canPaste?: boolean;
    canSelectAll?: boolean;
};

export type WebviewMenuParams = {
    linkURL?: string;
    mediaType?: string;
    srcURL?: string;
    isEditable?: boolean;
    selectionText?: string;
    editFlags?: WebviewEditFlags;
    canGoBack?: boolean;
    canGoForward?: boolean;
    misspelledWord?: string;
    dictionarySuggestions?: string[];
};

export type WebviewMenuAction =
    | "open-link"
    | "copy-link"
    | "save-image"
    | "cut"
    | "copy"
    | "paste"
    | "select-all"
    | "replace-misspelling"
    | "add-to-dictionary"
    | "back"
    | "forward"
    | "reload"
    | "inspect"
    | "separator";

export type WebviewMenuItemDesc = {
    action: WebviewMenuAction;
    label?: string;
    enabled?: boolean;
    value?: string;
};

const MaxSpellSuggestions = 5;

export function isExternalOpenableUrl(url: string): boolean {
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

function buildSpellSection(params: WebviewMenuParams): WebviewMenuItemDesc[] {
    if (!params.isEditable || isEmpty(params.misspelledWord)) {
        return [];
    }
    const suggestions = (params.dictionarySuggestions ?? []).slice(0, MaxSpellSuggestions);
    const section: WebviewMenuItemDesc[] = suggestions.map((value) => ({
        action: "replace-misspelling",
        label: value,
        value,
    }));
    if (suggestions.length === 0) {
        section.push({ action: "replace-misspelling", label: "No Guesses Found", enabled: false });
    }
    section.push({ action: "add-to-dictionary", label: "Add to Dictionary" });
    return section;
}

function isEmpty(s: string): boolean {
    return s == null || s === "";
}

function buildEditSection(params: WebviewMenuParams): WebviewMenuItemDesc[] {
    const flags = params.editFlags ?? {};
    if (params.isEditable) {
        return [
            { action: "cut", label: "Cut", enabled: !!flags.canCut },
            { action: "copy", label: "Copy", enabled: !!flags.canCopy },
            { action: "paste", label: "Paste", enabled: !!flags.canPaste },
            { action: "select-all", label: "Select All", enabled: !!flags.canSelectAll },
        ];
    }
    if (params.selectionText != null && params.selectionText.trim() !== "") {
        return [{ action: "copy", label: "Copy", enabled: true }];
    }
    return [];
}

export function buildWebviewMenuItems(params: WebviewMenuParams): WebviewMenuItemDesc[] {
    const linkSection: WebviewMenuItemDesc[] = [];
    if (isExternalOpenableUrl(params.linkURL)) {
        linkSection.push({ action: "open-link", label: "Open Link in External Browser" });
        linkSection.push({ action: "copy-link", label: "Copy Link Address" });
    }
    const imageSection: WebviewMenuItemDesc[] = [];
    if (params.mediaType === "image" && !isEmpty(params.srcURL)) {
        imageSection.push({ action: "save-image", label: "Save Image" });
    }
    const pageSection: WebviewMenuItemDesc[] = [
        { action: "back", label: "Back", enabled: !!params.canGoBack },
        { action: "forward", label: "Forward", enabled: !!params.canGoForward },
        { action: "reload", label: "Reload" },
    ];
    const inspectSection: WebviewMenuItemDesc[] = [{ action: "inspect", label: "Inspect Element" }];
    const sections = [
        linkSection,
        imageSection,
        buildSpellSection(params),
        buildEditSection(params),
        pageSection,
        inspectSection,
    ];
    const result: WebviewMenuItemDesc[] = [];
    for (const section of sections) {
        if (section.length === 0) {
            continue;
        }
        if (result.length > 0) {
            result.push({ action: "separator" });
        }
        result.push(...section);
    }
    return result;
}
