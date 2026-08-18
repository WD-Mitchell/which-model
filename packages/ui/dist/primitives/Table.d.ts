import type { ReactNode } from 'react';
export interface TableColumn {
    key: string;
    label: string;
    width?: string;
    align?: 'left' | 'center' | 'right';
    sortable?: boolean;
}
export interface TableSort {
    key: string;
    dir: 'asc' | 'desc';
}
export interface TableProps {
    columns: TableColumn[];
    sort: TableSort | null;
    onSort: (sort: TableSort) => void;
    rows: (sort: TableSort | null) => ReactNode;
    className?: string;
}
export declare function Table({ columns, sort, onSort, rows, className }: TableProps): import("react").JSX.Element;
