import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Tag.module.css';
export function Tag({ variant, size = 'badge', onClick, className, children }) {
    return (_jsx("span", { className: cx('tag', `tag-${variant}`, size === 'badge' ? styles.badge : styles.chip, className), onClick: onClick, children: children }));
}
