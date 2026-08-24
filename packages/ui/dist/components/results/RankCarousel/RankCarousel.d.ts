import type { RankedModel } from '@which-model/core';
export interface RankCarouselProps {
    items: RankedModel[];
    /** Controlled focus index; clamped for display only. */
    index: number;
    /** Fired by enabled chevrons only. */
    onIndex: (i: number) => void;
}
export declare function RankCarousel({ items, index, onIndex }: RankCarouselProps): import("react").JSX.Element;
