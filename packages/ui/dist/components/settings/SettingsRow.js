import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import styles from './settings.module.css';
export function SettingsRow({ label, description, control, children, className }) {
    return (_jsxs("div", { className: cx(styles.row, className), children: [_jsxs("div", { className: styles.rowInfo, children: [_jsx("span", { className: styles.rowLabel, children: label }), description !== undefined && _jsx("p", { className: styles.rowDescription, children: description })] }), (control !== undefined || children !== undefined) && (_jsx("div", { className: styles.rowControl, children: control ?? children }))] }));
}
