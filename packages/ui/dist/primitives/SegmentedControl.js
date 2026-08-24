import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useId } from 'react';
import { cx } from '../utils/cx';
import styles from './SegmentedControl.module.css';
export function SegmentedControl({ options, value, onChange, className }) {
    const name = useId();
    return (_jsx("div", { className: cx('seg', className), role: "radiogroup", children: options.map((option) => (_jsxs("label", { className: cx('seg-opt', styles.opt), children: [_jsx("input", { type: "radio", name: name, value: option.value, checked: value === option.value, onChange: () => {
                        if (value !== option.value)
                            onChange(option.value);
                    } }), option.label] }, option.value))) }));
}
