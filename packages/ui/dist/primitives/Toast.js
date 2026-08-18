import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import styles from './Toast.module.css';
export const TOAST_DURATION_MS = 2600;
const ToastContext = createContext(null);
export function ToastProvider(props) {
    const [message, setMessage] = useState(null);
    const timerRef = useRef(null);
    useEffect(() => {
        return () => {
            if (timerRef.current !== null)
                window.clearTimeout(timerRef.current);
        };
    }, []);
    const show = useCallback((next) => {
        setMessage(next);
        if (timerRef.current !== null)
            window.clearTimeout(timerRef.current);
        timerRef.current = window.setTimeout(() => {
            setMessage(null);
            timerRef.current = null;
        }, TOAST_DURATION_MS);
    }, []);
    const value = useMemo(() => ({ show }), [show]);
    return (_jsxs(ToastContext.Provider, { value: value, children: [props.children, message !== null && _jsx("div", { className: styles.toast, children: message })] }));
}
export function useToast() {
    const ctx = useContext(ToastContext);
    if (!ctx)
        throw new Error('useToast requires ToastProvider');
    return ctx;
}
