import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Button.module.css';
export function Button({ variant, size = 'md', disabled = false, onClick, className, children, }) {
    return (_jsx("button", { type: "button", className: cx('btn', `btn-${variant}`, size === 'sm' && styles.sm, className), disabled: disabled, onClick: onClick, children: children }));
}
