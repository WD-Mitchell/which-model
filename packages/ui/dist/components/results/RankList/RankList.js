import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../../utils/cx';
import styles from './RankList.module.css';
export function RankList({ items, index, onPick }) {
    return (_jsx("div", { className: styles.band, children: items.map((model, i) => {
            const selected = i === index;
            const provider = model.route_key.split('/')[0];
            return (_jsxs("button", { type: "button", "aria-current": selected ? 'true' : undefined, className: cx(styles.row, selected && styles.rowSelected), onClick: () => onPick(i), children: [_jsx("span", { className: styles.rank, children: model.rank }), _jsx("span", { className: cx(styles.name, selected && styles.nameSelected), children: model.model_name }), _jsx("span", { className: cx(styles.score, selected && styles.scoreSelected), children: model.score.toFixed(2) }), _jsx("span", { className: styles.route, children: provider })] }, model.route_key));
        }) }));
}
