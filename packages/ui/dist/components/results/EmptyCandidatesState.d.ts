export interface EmptyCandidatesStateProps {
    title?: string;
    message?: string;
    /** When both `actionLabel` and `onAction` are provided, a call-to-action renders. */
    actionLabel?: string;
    onAction?: () => void;
}
export declare function EmptyCandidatesState({ title, message, actionLabel, onAction, }: EmptyCandidatesStateProps): import("react").JSX.Element;
