import { jsxs as _jsxs, jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Table.module.css';
export function Table({ columns, sort, onSort, rows, className }) {
    return (_jsxs("div", { className: cx(className), children: [_jsx("div", { className: styles.headerRow, children: columns.map((column) => {
                    const active = sort !== null && sort.key === column.key;
                    const dir = active ? sort.dir : null;
                    const suffix = active ? (dir === 'desc' ? '  ↓' : '  ↑') : '';
                    return (_jsxs("span", { className: cx(styles.headerCell, styles[`align${column.align ?? 'left'}`], column.sortable && styles.sortable, active && styles.active), style: column.width ? { flex: 'none', width: column.width } : { flex: 1 }, onClick: column.sortable
                            ? () => onSort({ key: column.key, dir: dir === 'desc' ? 'asc' : 'desc' })
                            : undefined, children: [column.label, suffix] }, column.key));
                }) }), rows(sort)] }));
}
