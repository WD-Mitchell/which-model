export type WeightVariant = 'step' | 'bar' | 'slider';
export interface WeightRowProps {
    variant: WeightVariant;
    label: string;
    value: number;
    accent?: boolean;
    readOnly?: boolean;
    labelWidth?: 104 | 150;
    valueStyle?: 'compact' | 'verbose';
    /**
     * Lowest value the control can be dragged or keyed to.
     *
     * 1 wherever the weight must stay a weight: the engine rejects any weight
     * outside (0, 5] (internal/pick/profile.go rules 4 and 6), so a 0 there is
     * not "off", it is an unsaveable profile — and for a core axis it would
     * delete a key the engine requires. Editors that offer 0 use it as the
     * "ignored" gesture for a TASK benchmark they have no other way to drop.
     */
    min?: 0 | 1;
    onChange?: (v: number) => void;
    onRemove?: () => void;
}
export declare function WeightRow({ variant, label, value, accent, readOnly, labelWidth, valueStyle, min, onChange, onRemove, }: WeightRowProps): import("react").JSX.Element;
