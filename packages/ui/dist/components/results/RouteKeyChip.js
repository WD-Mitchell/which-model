import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import styles from './RouteKeyChip.module.css';
export function RouteKeyChip({ routeKey }) {
    const slash = routeKey.indexOf('/');
    const provider = slash === -1 ? routeKey : routeKey.slice(0, slash);
    const model = slash === -1 ? '' : routeKey.slice(slash);
    return (_jsxs("span", { className: styles.chip, title: routeKey, children: [_jsx("span", { className: styles.provider, children: provider }), _jsx("span", { className: styles.model, children: model })] }));
}
