import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../../utils/cx';
import styles from './RankCarousel.module.css';
import { MissingBenchmarkNotice } from '../MissingBenchmarkNotice';
function clampIndex(i, length) {
    if (length <= 0)
        return 0;
    return Math.max(0, Math.min(i, length - 1));
}
function formatScore(n) {
    if (n == null)
        return '—';
    return Number.isInteger(n) ? String(n) : n.toFixed(1);
}
export function RankCarousel({ items, index, onIndex }) {
    const idx = clampIndex(index, items.length);
    const model = items[idx];
    const rankLine = model ? `rank ${idx + 1} of ${items.length}` : 'no route';
    // Reasoning belongs to the model's identity, so it is bracketed after the
    // name in the same type rather than repeated in the meta line.
    const nameLine = model
        ? model.reasoning
            ? `${model.model_name} (${model.reasoning})`
            : model.model_name
        : 'Enable a provider';
    const metaLine = model
        ? `${model.provider} · ${model.score.toFixed(2)}`
        : 'every provider is switched off';
    // Prev disabled iff at the first item (empty list ⇒ only slot); next
    // disabled iff at the last item.
    const prevDisabled = idx <= 0;
    const nextDisabled = idx >= items.length - 1;
    return (_jsx("div", { className: styles.band, children: _jsxs("div", { className: styles.row, children: [_jsx("button", { type: "button", "aria-label": "previous rank", disabled: prevDisabled, className: cx(styles.chev, prevDisabled && styles.chevDisabled), onClick: () => onIndex(idx - 1), children: _jsx("svg", { width: "12", height: "12", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.7", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M7.2 2.2 3.4 6l3.8 3.8" }) }) }), _jsxs("span", { className: styles.center, children: [_jsx("span", { className: styles.rank, children: rankLine }), _jsx("span", { className: styles.name, children: nameLine }), _jsx("span", { className: styles.meta, children: metaLine }), model && (model.intelligence != null || model.cost != null || model.speed != null) ? (_jsxs("span", { className: styles.ratings, children: [_jsxs("span", { className: styles.ratingItem, children: [_jsx("span", { className: styles.ratingLabel, children: "intel" }), ' ', _jsx("span", { className: styles.ratingValue, children: formatScore(model.intelligence) })] }), _jsx("span", { className: styles.ratingDot, children: "\u00B7" }), _jsxs("span", { className: styles.ratingItem, children: [_jsx("span", { className: styles.ratingLabel, children: "cost" }), ' ', _jsx("span", { className: styles.ratingValue, children: formatScore(model.cost) })] }), _jsx("span", { className: styles.ratingDot, children: "\u00B7" }), _jsxs("span", { className: styles.ratingItem, children: [_jsx("span", { className: styles.ratingLabel, children: "speed" }), ' ', _jsx("span", { className: styles.ratingValue, children: formatScore(model.speed) })] })] })) : null, model ? _jsx(MissingBenchmarkNotice, { model: model }) : null] }), _jsx("button", { type: "button", "aria-label": "next rank", disabled: nextDisabled, className: cx(styles.chev, nextDisabled && styles.chevDisabled), onClick: () => onIndex(idx + 1), children: _jsx("svg", { width: "12", height: "12", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.7", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M4.8 2.2 8.6 6l-3.8 3.8" }) }) })] }) }));
}
