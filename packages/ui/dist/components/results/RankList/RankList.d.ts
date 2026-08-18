import type { RankedModel } from '@which-model/core';
export interface RankListProps {
    items: RankedModel[];
    /** Selected row (array index); controlled. */
    index: number;
    /** Fired on any row click, including the selected row. */
    onPick: (i: number) => void;
}
export declare function RankList({ items, index, onPick }: RankListProps): import("react").JSX.Element;
