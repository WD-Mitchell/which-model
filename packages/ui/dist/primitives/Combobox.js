import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useEffect, useRef } from 'react';
import { cx } from '../utils/cx';
import { Input } from './Input';
import styles from './Combobox.module.css';
export function Combobox({ items, query, onQuery, open, onOpenChange, onPick, emptyText, placeholder, selectedKey, }) {
    const rootRef = useRef(null);
    // Click-away closes. The list is not focus-managed (the input keeps focus),
    // so a blur handler would fire on every scroll and selection instead.
    useEffect(() => {
        if (!open)
            return;
        function onPointerDown(e) {
            if (rootRef.current && !rootRef.current.contains(e.target))
                onOpenChange(false);
        }
        window.addEventListener('pointerdown', onPointerDown);
        return () => window.removeEventListener('pointerdown', onPointerDown);
    }, [open, onOpenChange]);
    function handleKeyDown(e) {
        if (e.key === 'Enter') {
            if (items.length > 0)
                onPick(items[0].key);
        }
        else if (e.key === 'Escape' && open) {
            // The Combobox owns Escape ONLY while its list is open: close the list
            // and stop the event from reaching window-level dismiss listeners (the
            // settings shell closes the whole window otherwise — Menu.tsx does the
            // same). With the list closed, Escape bubbles so the shell can still
            // close the window (issue #65 review).
            e.stopPropagation();
            e.preventDefault();
            onOpenChange(false);
        }
    }
    return (
    // Deliberate interaction opens the list: a pointer-down on the field, or
    // typing (the consumer's onQuery opens it). NOT bare focus — the popover
    // window auto-focuses this input every time it is shown, and focus-opens
    // meant the full profile list popped up before the user touched anything.
    _jsxs("div", { ref: rootRef, className: styles.root, onPointerDown: () => onOpenChange(true), children: [_jsx(Input, { value: query, onChange: onQuery, placeholder: placeholder, onKeyDown: handleKeyDown }), open && (_jsx("div", { className: styles.surface, children: items.length === 0 ? (_jsx("div", { className: styles.empty, children: emptyText })) : (items.map((item) => (_jsxs("div", { className: cx(styles.row, item.key === selectedKey && styles.rowSelected), onClick: () => onPick(item.key), children: [_jsx("span", { className: styles.rowLabel, children: item.label }), _jsx("span", { className: styles.rowSub, children: item.sub })] }, item.key)))) }))] }));
}
