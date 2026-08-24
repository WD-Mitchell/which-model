import type { ReactNode } from 'react';
import { type WeightVariant } from './WeightRow';
export interface WeightEditorRow {
    key: string;
    value: number;
    accent?: boolean;
}
export interface WeightEditorProps {
    variant: 'popover' | 'profile-detail';
    sliderStyle: WeightVariant;
    coreRows: WeightEditorRow[];
    taskRows: WeightEditorRow[];
    sectionPcts: {
        core: string;
        task: string;
    };
    readOnly?: boolean;
    addable?: string[];
    addOpen?: boolean;
    onChangeWeight?: (key: string, v: number) => void;
    onRemoveWeight?: (key: string) => void;
    onAddMetric?: (key: string) => void;
    onToggleAdd?: () => void;
    onRevert?: () => void;
    /** Extra buttons in the action row, left of Revert. The popover puts its
     *  weights actions (Copy model id / Save as profile) here — they act on the
     *  weights, and the popover footer is now tab-independent. */
    extraActions?: ReactNode;
}
export declare function WeightEditor({ variant, sliderStyle, coreRows, taskRows, sectionPcts, readOnly, addable, addOpen, onChangeWeight, onRemoveWeight, onAddMetric, onToggleAdd, onRevert, extraActions, }: WeightEditorProps): import("react").JSX.Element;
