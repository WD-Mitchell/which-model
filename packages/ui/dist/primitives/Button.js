import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Button.module.css';
export function Button({ variant, size = 'md', disabled = false, onClick, className, children, }) {
    return (_jsx("button", { type: "button", 
        // U02 SPEC §2.5 — the look is nocturne's: `.btn` + `.btn-primary` /
        // `.btn-secondary` / `.btn-ghost` (transparent ground, accent text and
        // border for primary), never hand-rolled here, so retuning
        // theme/nocturne.css propagates to every caller.
        className: cx('btn', `btn-${variant}`, size !== 'md' && styles[size], className), disabled: disabled, onClick: onClick, children: children }));
}
