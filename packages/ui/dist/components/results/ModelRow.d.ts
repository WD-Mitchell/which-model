import type { RankedModel } from '@which-model/core';
import type { SparkbarEntry } from './Sparkbar';
export interface ModelRowProps {
    model: RankedModel;
    metrics: SparkbarEntry[];
    selected: boolean;
    onSelect: () => void;
    /** Launch action cell; omitted ⇒ the cell is left empty. */
    onLaunch?: () => void;
}
export declare function ModelRow({ model, metrics, selected, onSelect, onLaunch }: ModelRowProps): import("react").JSX.Element;
