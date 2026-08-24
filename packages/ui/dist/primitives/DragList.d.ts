import type React from 'react';
export interface DragListItem {
    id: string;
    node: React.ReactNode;
}
export interface DragListProps {
    items: DragListItem[];
    onReorder: (ids: string[]) => void;
    handle?: React.ReactNode;
    /** Merged onto every row, after the built-in classes. The row is the element
     *  the handle and the item node share, so a caller that needs the mockup's
     *  row rule and 22px gutter (demo.dc.html 745) puts them here rather than
     *  inside `node`, where they would leave the handle outside the padding. */
    rowClassName?: string;
}
export declare function DragList({ items, onReorder, handle, rowClassName }: DragListProps): React.JSX.Element;
