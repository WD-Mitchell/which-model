import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './SplitButton.module.css';
export function SplitButton({ label, onMain, menuItems, onPick, open, onOpenChange, disabled = false, }) {
    return (_jsxs("span", { className: styles.root, children: [_jsxs("span", { className: cx(styles.pill, open && styles.openBg, disabled && styles.disabled), children: [_jsx("span", { className: styles.label, onClick: disabled ? undefined : onMain, children: label }), _jsx("span", { className: styles.chevron, onClick: disabled ? undefined : () => onOpenChange(!open), "aria-expanded": open, role: "button", children: _jsx("svg", { width: "9", height: "9", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.6", strokeLinecap: "round", children: _jsx("path", { d: "M2.5 4.5 6 8l3.5-3.5" }) }) })] }), open && !disabled && (_jsx("span", { className: styles.surface, children: menuItems.map((item) => (_jsx("span", { className: cx(styles.item, item.selected && styles.itemSelected), onClick: () => onPick(item.key), children: item.label }, item.key))) }))] }));
}
