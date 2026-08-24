import type React from 'react';
export interface MenuItem {
    key: string;
    label?: string;
    separator?: boolean;
    dim?: boolean;
    mono?: boolean;
    selected?: boolean;
}
export interface MenuProps {
    items: MenuItem[];
    onPick: (key: string) => void;
    onClose: () => void;
    className?: string;
    /** Inline positioning for a context menu opened at the pointer, where the
     *  coordinates are only known at click time and cannot live in CSS. */
    style?: React.CSSProperties;
}
export declare function Menu({ items, onPick, onClose, className, style }: MenuProps): React.JSX.Element;
