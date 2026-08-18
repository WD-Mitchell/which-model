import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './UsageMeter.module.css';
export function UsageMeter({ label, percent, hot = false }) {
    const pct = percent === null ? 0 : Math.max(0, Math.min(100, percent));
    const fillClass = percent === null
        ? styles.fillNull
        : hot || percent >= 70
            ? styles.fillHot
            : styles.fillCool;
    return (_jsxs("div", { className: styles.meter, children: [_jsxs("span", { className: styles.labelLine, children: [_jsx("span", { className: styles.label, children: label }), _jsx("span", { className: styles.value, children: percent === null ? '—' : percent + '%' })] }), _jsx("span", { className: styles.track, children: _jsx("span", { className: cx(styles.fill, fillClass), style: { width: pct + '%' } }) })] }));
}
