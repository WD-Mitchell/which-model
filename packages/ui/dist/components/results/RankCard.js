import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import { SplitButton } from '../../primitives/SplitButton';
import { Sparkbar } from './Sparkbar';
import { RouteKeyChip } from './RouteKeyChip';
import styles from './RankCard.module.css';
function PinGlyph() {
    return (_jsx("svg", { width: "11", height: "11", viewBox: "0 0 12 12", fill: "none", stroke: "currentColor", strokeWidth: "1.5", strokeLinecap: "round", strokeLinejoin: "round", children: _jsx("path", { d: "M9 4.5 7.5 3M9 4.5l-1.7 1.7 1 .9-.9 2.2L4 5.4l2.2-.9.9 1L8.8 3.8 7.5 2.3 9 4.5Z" }) }));
}
export function RankCard({ model, rankLine, metrics, focused = false, pinned = false, onTogglePin, launchLabel, harnesses, launchMenuOpen = false, onLaunchMenuOpenChange, onLaunch, onHarnessChange, }) {
    const meta = `${model.provider} · ${model.reasoning} · ${model.score.toFixed(2)}`;
    const hasLauncher = Boolean(launchLabel && harnesses && onLaunch && onLaunchMenuOpenChange);
    return (_jsxs("article", { className: cx(styles.card, focused && styles.focused), children: [_jsxs("div", { className: styles.heading, "aria-live": "polite", children: [_jsx("span", { className: styles.rank, children: rankLine }), _jsx("span", { className: styles.name, children: model.model_name }), _jsx("span", { className: styles.meta, children: meta })] }), _jsxs("div", { className: styles.detailRow, children: [_jsx(RouteKeyChip, { routeKey: model.route_key }), _jsx("span", { className: styles.score, children: model.score.toFixed(2) }), onTogglePin && (_jsx("button", { type: "button", className: cx(styles.pin, pinned && styles.pinActive), onClick: onTogglePin, "aria-pressed": pinned, "aria-label": pinned ? 'unpin model' : 'pin model', children: _jsx(PinGlyph, {}) }))] }), _jsx(Sparkbar, { metrics: metrics }), hasLauncher && (_jsx("div", { className: styles.footer, children: _jsx(SplitButton, { label: launchLabel, onMain: onLaunch, menuItems: harnesses, onPick: onHarnessChange, open: launchMenuOpen, onOpenChange: onLaunchMenuOpenChange }) }))] }));
}
