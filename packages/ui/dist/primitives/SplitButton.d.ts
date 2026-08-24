import type React from 'react';
export interface SplitButtonMenuItem {
    key: string;
    label: string;
    selected: boolean;
}
export interface SplitButtonProps {
    label: string;
    onMain: () => void;
    menuItems: SplitButtonMenuItem[];
    onPick: (key: string) => void;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}
export declare function SplitButton({ label, onMain, menuItems, onPick, open, onOpenChange, }: SplitButtonProps): React.JSX.Element;
