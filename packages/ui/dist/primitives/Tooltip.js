import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useState } from 'react';
import styles from './Tooltip.module.css';
export function Tooltip({ content, children }) {
    const [shown, setShown] = useState(false);
    return (_jsxs("span", { className: styles.wrap, onMouseEnter: () => setShown(true), onMouseLeave: () => setShown(false), children: [children, shown && _jsx("span", { className: styles.surface, children: content })] }));
}
