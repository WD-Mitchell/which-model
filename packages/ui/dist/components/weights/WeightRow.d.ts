export type WeightVariant = 'step' | 'bar' | 'slider';
export interface WeightRowProps {
    variant: WeightVariant;
    label: string;
    value: number;
    accent?: boolean;
    readOnly?: boolean;
    labelWidth?: 104 | 150;
    valueStyle?: 'compact' | 'verbose';
    onChange?: (v: number) => void;
    onRemove?: () => void;
}
export declare function WeightRow({ variant, label, value, accent, readOnly, labelWidth, valueStyle, onChange, onRemove, }: WeightRowProps): import("react").JSX.Element;
