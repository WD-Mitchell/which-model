import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import { Input } from './Input';
import styles from './Combobox.module.css';
export function Combobox({ items, query, onQuery, open, onOpenChange, onPick, emptyText, placeholder, selectedKey, }) {
    function handleKeyDown(e) {
        if (e.key === 'Enter') {
            if (items.length > 0)
                onPick(items[0].key);
        }
        else if (e.key === 'Escape') {
            onOpenChange(false);
        }
    }
    return (_jsxs("div", { className: styles.root, children: [_jsx(Input, { value: query, onChange: onQuery, placeholder: placeholder, onFocus: () => onOpenChange(true), onKeyDown: handleKeyDown }), open && (_jsx("div", { className: styles.surface, children: items.length === 0 ? (_jsx("div", { className: styles.empty, children: emptyText })) : (items.map((item) => (_jsxs("div", { className: cx(styles.row, item.key === selectedKey && styles.rowSelected), onClick: () => onPick(item.key), children: [_jsx("span", { className: styles.rowLabel, children: item.label }), _jsx("span", { className: styles.rowSub, children: item.sub })] }, item.key)))) }))] }));
}
