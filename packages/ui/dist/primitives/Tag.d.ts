import type React from 'react';
export interface TagProps {
    variant: 'accent' | 'neutral' | 'outline';
    size?: 'badge' | 'chip';
    onClick?: () => void;
    className?: string;
    children: React.ReactNode;
}
export declare function Tag({ variant, size, onClick, className, children }: TagProps): React.JSX.Element;
