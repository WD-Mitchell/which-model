export interface UsageMeterProps {
    label: string;
    percent: number | null;
    hot?: boolean;
}
export declare function UsageMeter({ label, percent, hot }: UsageMeterProps): import("react").JSX.Element;
