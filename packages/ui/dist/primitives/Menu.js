import { jsx as _jsx } from "react/jsx-runtime";
import { useEffect, useRef } from 'react';
import { cx } from '../utils/cx';
import styles from './Menu.module.css';
export function Menu({ items, onPick, onClose, className, style }) {
    const ref = useRef(null);
    useEffect(() => {
        function onPointerDown(e) {
            if (ref.current && !ref.current.contains(e.target))
                onClose();
        }
        function onKeyDown(e) {
            if (e.key === 'Escape') {
                e.stopPropagation();
                onClose();
            }
        }
        window.addEventListener('pointerdown', onPointerDown);
        window.addEventListener('keydown', onKeyDown);
        return () => {
            window.removeEventListener('pointerdown', onPointerDown);
            window.removeEventListener('keydown', onKeyDown);
        };
    }, [onClose]);
    return (_jsx("div", { ref: ref, role: "menu", className: cx(styles.surface, className), style: style, children: items.map((item) => item.separator ? (_jsx("div", { role: "separator", className: styles.separator }, item.key)) : (_jsx("div", { role: "menuitem", className: cx(styles.item, item.dim && styles.dim, item.mono && styles.mono, item.selected && styles.selected), onClick: () => onPick(item.key), children: item.label }, item.key))) }));
}
