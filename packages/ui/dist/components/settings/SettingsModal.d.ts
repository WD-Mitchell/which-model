import { type ReactNode } from 'react';
export interface SettingsModalProps {
    /** Visibility is controlled; omitted defaults to visible for simple confirmation usage. */
    open?: boolean;
    /** Alias for `open`. */
    isOpen?: boolean;
    title?: ReactNode;
    description?: ReactNode;
    children?: ReactNode;
    /** Render custom footer controls instead of the convenience confirm/cancel controls. */
    actions?: ReactNode;
    onClose?: () => void;
    /** Optional form submit callback for create/edit forms. */
    onSubmit?: () => void;
    onConfirm?: () => void;
    confirmLabel?: string;
    cancelLabel?: string;
    confirmDisabled?: boolean;
    closeOnBackdrop?: boolean;
    className?: string;
}
export declare function SettingsModal({ open, isOpen, title, description, children, actions, onClose, onSubmit, onConfirm, confirmLabel, cancelLabel, confirmDisabled, closeOnBackdrop, className, }: SettingsModalProps): import("react").JSX.Element | null;
