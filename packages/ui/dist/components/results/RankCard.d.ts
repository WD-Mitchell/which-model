import type { RankedModel } from '@which-model/core';
import type { SplitButtonMenuItem } from '../../primitives/SplitButton';
import type { SparkbarEntry } from './Sparkbar';
export interface RankCardProps {
    model: RankedModel;
    /** Rank line, e.g. `rank 1 of 3`. */
    rankLine: string;
    /** Core metric bars (intelligence/speed/cost). */
    metrics: SparkbarEntry[];
    /** Highlight as the focused/selected card of a carousel. */
    focused?: boolean;
    /** Favourite/pin state; supplying `onTogglePin` renders the toggle. */
    pinned?: boolean;
    onTogglePin?: () => void;
    /** Launch split-button group; all four are required together to render. */
    launchLabel?: string;
    harnesses?: SplitButtonMenuItem[];
    launchMenuOpen?: boolean;
    onLaunchMenuOpenChange?: (open: boolean) => void;
    onLaunch?: () => void;
    onHarnessChange?: (key: string) => void;
}
export declare function RankCard({ model, rankLine, metrics, focused, pinned, onTogglePin, launchLabel, harnesses, launchMenuOpen, onLaunchMenuOpenChange, onLaunch, onHarnessChange, }: RankCardProps): import("react").JSX.Element;
