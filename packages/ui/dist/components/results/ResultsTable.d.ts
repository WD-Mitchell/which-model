import type { RankedModel } from '@which-model/core';
import type { SparkbarEntry } from './Sparkbar';
export interface ResultsTableProps {
    /** Rank-ascending candidates as delivered by the backend. */
    items: RankedModel[];
    /** Per-model core metric bars. */
    metrics: (model: RankedModel) => SparkbarEntry[];
    /** Index of the selected row. */
    selectedIndex: number;
    onSelect: (index: number) => void;
    /** Launch action column; omitted ⇒ every row's launch cell is empty. */
    onLaunch?: (model: RankedModel) => void;
}
export declare function ResultsTable({ items, metrics, selectedIndex, onSelect, onLaunch, }: ResultsTableProps): import("react").JSX.Element;
