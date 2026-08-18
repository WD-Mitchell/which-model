import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './CoverageBar.module.css';
export function CoverageBar({ covered, total, className }) {
    const pct = total <= 0 ? 0 : Math.round((covered / total) * 100);
    return (_jsx("span", { className: cx(styles.track, className), children: _jsx("span", { className: styles.fill, style: { width: pct + '%' } }) }));
}
