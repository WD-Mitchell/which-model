import type React from 'react';
export interface InputProps {
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    mono?: boolean;
    disabled?: boolean;
    className?: string;
    onFocus?: () => void;
    onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void;
}
export declare function Input({ value, onChange, placeholder, mono, disabled, className, onFocus, onKeyDown, }: InputProps): React.JSX.Element;
