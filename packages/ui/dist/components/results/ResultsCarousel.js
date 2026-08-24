import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useRef, useState } from 'react';
import { cx } from '../../utils/cx';
import { RankCard } from './RankCard';
import styles from './ResultsCarousel.module.css';
function clampIndex(i, length) {
    if (length === 0)
        return 0;
    return Math.max(0, Math.min(i, length - 1));
}
export function ResultsCarousel({ items, metrics, index, onIndex, rankLabel, pinned, onTogglePin, harnessNames, selectedHarness = '', launchLabel, onLaunch, onHarnessChange, }) {
    const lastIndex = items.length - 1;
    const shown = clampIndex(index, items.length);
    // Carousel card refs, one per item, for scrolling the focused card into view.
    const cardRefs = useRef([]);
    // Launch menu open is ephemeral popover state (mockup harnessMenuOpen).
    const [openMenu, setOpenMenu] = useState(null);
    useEffect(() => {
        const el = cardRefs.current[shown];
        if (el && typeof el.scrollIntoView === 'function') {
            el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        }
    }, [shown]);
    const menuItems = harnessNames?.map((name) => ({
        key: name,
        label: name,
        selected: name === selectedHarness,
    }));
    const hasLauncher = Boolean(menuItems && onLaunch && harnessNames && harnessNames.length > 0);
    return (_jsxs("div", { className: styles.root, children: [_jsx("div", { className: styles.viewport, children: items.map((model, i) => {
                    const interactive = hasLauncher
                        ? {
                            launchLabel: launchLabel?.(model) ?? `Launch in ${selectedHarness}`,
                            harnesses: menuItems,
                            launchMenuOpen: openMenu === i,
                            onLaunchMenuOpenChange: (open) => setOpenMenu(open ? i : null),
                            onLaunch: () => onLaunch?.(model),
                            onHarnessChange: (key) => {
                                setOpenMenu(null);
                                onHarnessChange?.(model, key);
                            },
                        }
                        : {};
                    return (_jsx("div", { ref: (el) => {
                            cardRefs.current[i] = el;
                        }, className: styles.cardSlot, children: _jsx(RankCard, { model: model, rankLine: rankLabel(i, items.length), metrics: metrics(model), focused: i === shown, pinned: pinned ? pinned(model) : false, onTogglePin: onTogglePin ? () => onTogglePin(model) : undefined, ...interactive }) }, model.model_id));
                }) }), _jsxs("div", { className: styles.controls, children: [_jsx("button", { type: "button", className: styles.chevron, "aria-label": "previous rank", disabled: shown === 0, onClick: () => onIndex(clampIndex(shown - 1, items.length)), children: _jsx("svg", { width: "12", height: "12", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.7", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M7.2 2.2 3.4 6l3.8 3.8" }) }) }), _jsx("div", { className: styles.dots, role: "tablist", "aria-label": "results", children: items.map((model, i) => (_jsx("button", { type: "button", className: cx(styles.dot, i === shown && styles.dotActive), "aria-label": `go to rank ${i + 1}`, "aria-current": i === shown ? 'true' : undefined, onClick: () => onIndex(i) }, model.model_id))) }), _jsx("button", { type: "button", className: styles.chevron, "aria-label": "next rank", disabled: shown >= lastIndex, onClick: () => onIndex(clampIndex(shown + 1, items.length)), children: _jsx("svg", { width: "12", height: "12", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.7", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M4.8 2.2 8.6 6l-3.8 3.8" }) }) })] })] }));
}
