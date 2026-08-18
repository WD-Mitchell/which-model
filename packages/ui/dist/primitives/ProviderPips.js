import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './ProviderPips.module.css';
export function ProviderPips({ states }) {
    return (_jsx("span", { className: styles.row, children: states.map((on, i) => (_jsx("span", { className: cx(styles.pip, on && styles.on) }, i))) }));
}
