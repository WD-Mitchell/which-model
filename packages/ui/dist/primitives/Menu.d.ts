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
}
export declare function Menu({ items, onPick, onClose, className }: MenuProps): import("react").JSX.Element;
