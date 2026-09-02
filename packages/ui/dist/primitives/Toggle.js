import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
export function Toggle({ on, disabled = false, onToggle, 'aria-label': ariaLabel }) {
    function toggle() {
        if (!disabled)
            onToggle(!on);
    }
    function handleKey(e) {
        if (disabled)
            return;
        if (e.key === ' ' || e.key === 'Enter') {
            e.preventDefault();
            onToggle(!on);
        }
    }
    return (_jsx("span", { role: "switch", "aria-checked": on, "aria-label": ariaLabel, tabIndex: disabled ? -1 : 0, className: cx('sw', on && 'on', disabled && 'off'), onClick: toggle, onKeyDown: handleKey, children: _jsx("i", {}) }));
}
