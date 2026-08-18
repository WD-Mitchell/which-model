import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import { Tag } from '../../primitives/Tag';
import { Sparkbar } from './Sparkbar';
import styles from './ModelRow.module.css';
export function ModelRow({ model, metrics, selected, onSelect, onLaunch }) {
    function onKeyDown(e) {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onSelect();
        }
    }
    return (_jsxs("tr", { className: cx(styles.row, selected && styles.selected), "aria-selected": selected, tabIndex: 0, onClick: onSelect, onKeyDown: onKeyDown, children: [_jsx("td", { className: cx(styles.cell, styles.rank), children: model.rank }), _jsxs("td", { className: cx(styles.cell, styles.nameCell), children: [_jsx("span", { className: styles.name, children: model.model_name }), _jsx("span", { className: styles.modelId, children: model.model_id })] }), _jsx("td", { className: styles.cell, children: model.provider }), _jsx("td", { className: styles.cell, children: _jsx(Tag, { variant: "outline", size: "badge", children: model.reasoning }) }), _jsx("td", { className: cx(styles.cell, styles.score), children: model.score.toFixed(2) }), _jsx("td", { className: styles.cell, children: _jsx(Sparkbar, { metrics: metrics, label: false }) }), _jsx("td", { className: cx(styles.cell, styles.launch), children: onLaunch && (_jsx("button", { type: "button", className: styles.launchBtn, "aria-label": `launch ${model.model_name}`, onClick: (e) => {
                        e.stopPropagation();
                        onLaunch();
                    }, children: "Launch" })) })] }));
}
