export interface SegmentedOption {
    value: string;
    label: string;
}
export interface SegmentedControlProps {
    options: SegmentedOption[];
    value: string;
    onChange: (value: string) => void;
    className?: string;
}
export declare function SegmentedControl({ options, value, onChange, className }: SegmentedControlProps): import("react").JSX.Element;
