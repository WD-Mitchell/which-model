export interface BalanceSliderProps {
    core: number;
    readOnly?: boolean;
    showRatio?: boolean;
    onChange?: (v: number) => void;
}
export declare function BalanceSlider({ core, readOnly, showRatio, onChange }: BalanceSliderProps): import("react").JSX.Element;
