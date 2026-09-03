import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Input.module.css';
export function Input({ value, onChange, placeholder, mono = true, disabled = false, className, type = 'text', 'aria-label': ariaLabel, onFocus, onBlur, onKeyDown, }) {
    return (_jsx("span", { className: cx('input', styles.wrap, className), children: _jsx("input", { className: cx('wminput', !mono && styles.body), type: type, value: value, placeholder: placeholder, disabled: disabled, "aria-label": ariaLabel, onChange: (e) => onChange(e.target.value), onFocus: onFocus, onBlur: onBlur, onKeyDown: onKeyDown }) }));
}
