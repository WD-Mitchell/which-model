import { jsx as _jsx } from "react/jsx-runtime";
import { cx } from '../utils/cx';
import styles from './Input.module.css';
export function Input({ value, onChange, placeholder, mono = true, disabled = false, className, onFocus, onKeyDown, }) {
    return (_jsx("span", { className: cx('input', styles.wrap, className), children: _jsx("input", { className: cx('wminput', !mono && styles.body), value: value, placeholder: placeholder, disabled: disabled, onChange: (e) => onChange(e.target.value), onFocus: onFocus, onKeyDown: onKeyDown }) }));
}
