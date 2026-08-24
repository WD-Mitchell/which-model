import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Tag.module.css';
export function Tag({ variant, size = 'badge', mono = true, onClick, className, children, }) {
    return (_jsx("span", { className: cx(mono && 'mono', 'tag', `tag-${variant}`, size !== 'md' && styles[size], className), onClick: onClick, children: children }));
}
