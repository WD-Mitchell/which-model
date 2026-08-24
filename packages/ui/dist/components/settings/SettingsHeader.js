import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import styles from './settings.module.css';
function isActionObject(action) {
    if (typeof action !== 'object' || action === null || !('label' in action))
        return false;
    return typeof action.label === 'string';
}
function renderAction(action) {
    if (!isActionObject(action))
        return action;
    return (_jsx("button", { type: "button", className: styles.headerAction, disabled: action.disabled, onClick: action.onAction ?? action.onClick, children: action.label }));
}
export function SettingsHeader({ title, description, blurb, actions, action, actionLabel, onAction, className, }) {
    const supportingCopy = description ?? blurb;
    const convenienceAction = actionLabel
        ? { label: actionLabel, onAction }
        : undefined;
    const actionSlot = action ?? convenienceAction;
    return (_jsxs("header", { className: cx(styles.header, className), children: [_jsxs("div", { className: styles.headerCopy, children: [_jsx("h1", { className: styles.headerTitle, children: title }), supportingCopy !== undefined && _jsx("p", { className: styles.headerDescription, children: supportingCopy })] }), (actions !== undefined || actionSlot !== undefined) && (_jsxs("div", { className: styles.headerActions, children: [actions, actionSlot !== undefined && renderAction(actionSlot)] }))] }));
}
