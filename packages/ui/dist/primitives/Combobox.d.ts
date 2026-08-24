import type React from 'react';
export interface ComboboxItem {
    key: string;
    label: string;
    sub: string;
}
export interface ComboboxProps {
    items: ComboboxItem[];
    query: string;
    onQuery: (query: string) => void;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onPick: (key: string) => void;
    emptyText: string;
    placeholder?: string;
    selectedKey?: string;
}
export declare function Combobox({ items, query, onQuery, open, onOpenChange, onPick, emptyText, placeholder, selectedKey, }: ComboboxProps): React.JSX.Element;
