import type React from 'react';
export interface DragListItem {
    id: string;
    node: React.ReactNode;
}
export interface DragListProps {
    items: DragListItem[];
    onReorder: (ids: string[]) => void;
    handle?: React.ReactNode;
}
export declare function DragList({ items, onReorder, handle }: DragListProps): React.JSX.Element;
