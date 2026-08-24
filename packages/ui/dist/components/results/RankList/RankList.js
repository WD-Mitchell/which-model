import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../../utils/cx';
import styles from './RankList.module.css';
export function RankList({ items, index, onPick }) {
    return (_jsx("div", { className: styles.band, children: items.map((model, i) => {
            const selected = i === index;
            // Reasoning reads as part of the model's identity, so it sits in
            // brackets right after the name in the same type — not off in the route
            // column. It is load-bearing: the same model routinely holds several
            // adjacent ranks at different efforts, and the name alone renders them
            // as duplicate rows.
            const provider = model.route_key.split('/')[0];
            const name = model.reasoning ? `${model.model_name} (${model.reasoning})` : model.model_name;
            return (_jsxs("button", { type: "button", "aria-current": selected ? 'true' : undefined, className: cx(styles.row, selected && styles.rowSelected), onClick: () => onPick(i), children: [_jsx("span", { className: styles.rank, children: model.rank }), _jsx("span", { className: cx(styles.name, selected && styles.nameSelected), children: name }), _jsx("span", { className: cx(styles.score, selected && styles.scoreSelected), children: model.score.toFixed(2) }), _jsx("span", { className: styles.route, children: provider })] }, model.route_key));
        }) }));
}
