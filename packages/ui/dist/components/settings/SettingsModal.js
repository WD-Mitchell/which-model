import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { useEffect } from 'react';
import { cx } from '../../utils/cx';
import styles from './settings.module.css';
export function SettingsModal({ open, isOpen, title, description, children, actions, onClose, onSubmit, onConfirm, confirmLabel = 'Save', cancelLabel = 'Cancel', confirmDisabled = false, closeOnBackdrop = true, className, }) {
    const visible = isOpen ?? open ?? true;
    useEffect(() => {
        if (!visible || !onClose)
            return;
        const onKeyDown = (event) => {
            if (event.key === 'Escape')
                onClose();
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [onClose, visible]);
    if (!visible)
        return null;
    const hasConvenienceActions = onConfirm !== undefined || onSubmit !== undefined || onClose !== undefined;
    const submit = onSubmit ?? onConfirm;
    function handleSubmit(event) {
        event.preventDefault();
        submit?.();
    }
    function handleBackdropClick(event) {
        if (closeOnBackdrop && event.target === event.currentTarget)
            onClose?.();
    }
    return (_jsx("div", { className: styles.modalBackdrop, onMouseDown: handleBackdropClick, children: _jsxs("div", { className: cx(styles.modal, className), role: "dialog", "aria-modal": "true", "aria-labelledby": title !== undefined ? 'settings-modal-title' : undefined, children: [(title !== undefined || onClose !== undefined) && (_jsxs("header", { className: styles.modalHeader, children: [title !== undefined && (_jsx("h2", { className: styles.modalTitle, id: "settings-modal-title", children: title })), onClose !== undefined && (_jsx("button", { type: "button", className: styles.modalClose, "aria-label": "Close", onClick: onClose, children: "\u00D7" }))] })), description !== undefined && _jsx("p", { className: styles.modalDescription, children: description }), _jsxs("form", { onSubmit: handleSubmit, children: [_jsx("div", { className: styles.modalBody, children: children }), (actions !== undefined || hasConvenienceActions) && (_jsxs("div", { className: styles.modalActions, children: [actions, actions === undefined && (_jsxs(_Fragment, { children: [onClose !== undefined && (_jsx("button", { type: "button", className: cx(styles.modalButton, styles.modalButtonSecondary), onClick: onClose, children: cancelLabel })), submit !== undefined && (_jsx("button", { type: "submit", className: cx(styles.modalButton, styles.modalButtonPrimary), disabled: confirmDisabled, children: confirmLabel }))] }))] }))] })] }) }));
}
