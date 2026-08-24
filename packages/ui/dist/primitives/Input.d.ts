import type React from 'react';
export interface InputProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    mono?: boolean;
    disabled?: boolean;
    className?: string;
    onFocus?: () => void;
    /** Commit-on-blur editors (the mockup's inline rename fields, demo.dc.html
     *  462) need the counterpart to onFocus. */
    onBlur?: () => void;
    onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}
export declare function Input({ value, onChange, placeholder, mono, disabled, className, onFocus, onBlur, onKeyDown, }: InputProps): React.JSX.Element;
