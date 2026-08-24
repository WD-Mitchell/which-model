import type React from 'react';
export interface ButtonProps {
    variant: 'primary' | 'secondary' | 'ghost';
    /** 'md' = nocturne metrics; 'sm' = mockup footer scale (12px / 5px 11px,
     *  demo.dc.html 219–220, 274); 'xs' = the inline ghost scale the settings
     *  pages use (11px / 2px 7px, demo.dc.html 330, 462, 539–540, 760–761). */
    size?: 'md' | 'sm' | 'xs';
    disabled?: boolean;
    onClick?: () => void;
    className?: string;
    children: React.ReactNode;
}
export declare function Button({ variant, size, disabled, onClick, className, children, }: ButtonProps): React.JSX.Element;
