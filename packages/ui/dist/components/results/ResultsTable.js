import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { ModelRow } from './ModelRow';
import styles from './ResultsTable.module.css';
export function ResultsTable({ items, metrics, selectedIndex, onSelect, onLaunch, }) {
    return (_jsxs("table", { className: styles.table, children: [_jsx("thead", { children: _jsxs("tr", { children: [_jsx("th", { scope: "col", className: styles.th, children: "#" }), _jsx("th", { scope: "col", className: styles.th, children: "Model" }), _jsx("th", { scope: "col", className: styles.th, children: "Provider" }), _jsx("th", { scope: "col", className: styles.th, children: "Reasoning" }), _jsx("th", { scope: "col", className: styles.thRight, children: "Score" }), _jsx("th", { scope: "col", className: styles.th, children: "Metrics" }), _jsx("th", { scope: "col", className: styles.thLaunch })] }) }), _jsx("tbody", { children: items.map((model, i) => (_jsx(ModelRow, { model: model, metrics: metrics(model), selected: i === selectedIndex, onSelect: () => onSelect(i), onLaunch: onLaunch ? () => onLaunch(model) : undefined }, model.model_id))) })] }));
}
