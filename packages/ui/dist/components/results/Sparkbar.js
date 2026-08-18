import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Tooltip } from '../../primitives/Tooltip';
import styles from './Sparkbar.module.css';
/** round(4 + value/5*20) px, value clamped to 0..5 → 1→8, 3→16, 5→24. */
export function sparkbarHeight(value) {
    const v = Math.max(0, Math.min(5, value));
    return Math.round(4 + (v / 5) * 20);
}
export function Sparkbar({ metrics, label = true }) {
    return (_jsx("div", { className: styles.strip, role: "group", "aria-label": "core metrics", children: metrics.map(({ key, value }) => {
            const tip = `${key}  ${value} / 5`;
            return (_jsx(Tooltip, { content: tip, children: _jsxs("span", { className: styles.col, role: "img", "aria-label": tip, children: [_jsx("span", { className: styles.bar, style: { height: sparkbarHeight(value) } }), label && _jsx("span", { className: styles.tag, children: key })] }) }, key));
        }) }));
}
