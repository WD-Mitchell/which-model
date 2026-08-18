/** A single metric axis: `value` is on the 0..5 weight scale (clamped for height). */
export interface SparkbarEntry {
    key: string;
    value: number;
}
export interface SparkbarProps {
    /** Metric bars rendered in the order given (e.g. intelligence, speed, cost). */
    metrics: SparkbarEntry[];
    /** Show the metric key as a small label beneath each bar (axis reading). */
    label?: boolean;
}
/** round(4 + value/5*20) px, value clamped to 0..5 → 1→8, 3→16, 5→24. */
export declare function sparkbarHeight(value: number): number;
export declare function Sparkbar({ metrics, label }: SparkbarProps): import("react").JSX.Element;
