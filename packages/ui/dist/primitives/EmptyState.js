import { jsx as _jsx } from "react/jsx-runtime";
import styles from './EmptyState.module.css';
export function EmptyState({ text }) {
    return _jsx("div", { className: styles.empty, children: text });
}
