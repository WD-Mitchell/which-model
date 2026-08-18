import type React from 'react';
export interface ButtonProps {
    variant: 'primary' | 'secondary' | 'ghost';
    size?: 'md' | 'sm';
    disabled?: boolean;
    onClick?: () => void;
    className?: string;
    children: React.ReactNode;
}
export declare function Button({ variant, size, disabled, onClick, className, children, }: ButtonProps): React.JSX.Element;
