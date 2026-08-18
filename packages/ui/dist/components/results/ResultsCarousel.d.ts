import type { RankedModel } from '@which-model/core';
import type { SparkbarEntry } from './Sparkbar';
export interface ResultsCarouselProps {
    items: RankedModel[];
    /** Per-model core metric bars. */
    metrics: (model: RankedModel) => SparkbarEntry[];
    /** Controlled focus index; clamped for display only. */
    index: number;
    onIndex: (index: number) => void;
    /** Rank line per card, e.g. (i, total) => `rank ${i + 1} of ${total}`. */
    rankLabel: (index: number, total: number) => string;
    /** Favourite/pin state per model; absent ⇒ cards render no pin toggle. */
    pinned?: (model: RankedModel) => boolean;
    onTogglePin?: (model: RankedModel) => void;
    /** Harness choices for the launch split button; omitted ⇒ no launcher on cards. */
    harnessNames?: string[];
    selectedHarness?: string;
    launchLabel?: (model: RankedModel) => string;
    onLaunch?: (model: RankedModel) => void;
    onHarnessChange?: (model: RankedModel, harness: string) => void;
}
export declare function ResultsCarousel({ items, metrics, index, onIndex, rankLabel, pinned, onTogglePin, harnessNames, selectedHarness, launchLabel, onLaunch, onHarnessChange, }: ResultsCarouselProps): import("react").JSX.Element;
