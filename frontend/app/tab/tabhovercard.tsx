// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

import {
    FloatingPortal,
    autoUpdate,
    flip,
    offset,
    safePolygon,
    shift,
    useDismiss,
    useFloating,
    useHover,
    useInteractions,
    type Placement,
} from "@floating-ui/react";
import { useCallback, useEffect, useState } from "react";

const HoverOpenDelayMs = 500;

// The card renders in a portal but React still bubbles its events to the tab, which would select or drag it.
function stopBubble(e: React.SyntheticEvent) {
    e.stopPropagation();
}

type UseTabHoverCardOpts = {
    name: string;
    description?: string | null;
    disabled?: boolean;
    placement: Placement;
};

export function useTabHoverCard({ name, description, disabled, placement }: UseTabHoverCardOpts) {
    const [isOpen, setIsOpen] = useState(false);
    const { refs, floatingStyles, context } = useFloating({
        open: isOpen && !disabled,
        onOpenChange: setIsOpen,
        placement,
        middleware: [offset(6), flip(), shift({ padding: 8 })],
        whileElementsMounted: autoUpdate,
    });
    const hover = useHover(context, {
        delay: { open: HoverOpenDelayMs, close: 0 },
        move: false,
        handleClose: safePolygon(),
        enabled: !disabled,
    });
    const dismiss = useDismiss(context, { referencePress: true });
    const { getReferenceProps, getFloatingProps } = useInteractions([hover, dismiss]);

    useEffect(() => {
        if (disabled) {
            setIsOpen(false);
        }
    }, [disabled]);

    const closeCard = useCallback(() => setIsOpen(false), []);

    const card =
        isOpen && !disabled ? (
            <FloatingPortal>
                <div
                    ref={refs.setFloating}
                    style={floatingStyles}
                    {...getFloatingProps()}
                    className="z-50 max-w-[320px] rounded-md border border-border bg-zinc-800 px-2.5 py-1.5 text-xs text-foreground shadow-xl"
                    data-testid="tab-hover-card"
                    onMouseDown={stopBubble}
                    onClick={stopBubble}
                    onDoubleClick={stopBubble}
                    onContextMenu={stopBubble}
                >
                    <TabHoverCardContent name={name} description={description} />
                </div>
            </FloatingPortal>
        ) : null;

    return { setReference: refs.setReference, getReferenceProps, closeCard, card };
}

type TabHoverCardContentProps = {
    name: string;
    description?: string | null;
};

export function TabHoverCardContent({ name, description }: TabHoverCardContentProps) {
    return (
        <>
            <div className="font-bold break-words whitespace-pre-wrap">{name}</div>
            {description && (
                <div className="mt-1 max-h-[200px] overflow-y-auto break-words whitespace-pre-wrap text-secondary">
                    {description}
                </div>
            )}
        </>
    );
}
