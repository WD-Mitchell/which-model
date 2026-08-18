import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Button } from '../../primitives/Button';
import styles from './EmptyCandidatesState.module.css';
const DEFAULT_TITLE = 'No models match your routes';
const DEFAULT_MESSAGE = 'Switch on a provider or adjust your weights to see ranked candidates here.';
export function EmptyCandidatesState({ title = DEFAULT_TITLE, message = DEFAULT_MESSAGE, actionLabel, onAction, }) {
    const showAction = Boolean(actionLabel && onAction);
    return (_jsxs("div", { className: styles.empty, role: "status", children: [_jsx("span", { className: styles.kicker, children: "no routes" }), _jsx("span", { className: styles.title, children: title }), _jsx("span", { className: styles.message, children: message }), showAction && (_jsx(Button, { variant: "secondary", size: "sm", onClick: onAction, children: actionLabel }))] }));
}
