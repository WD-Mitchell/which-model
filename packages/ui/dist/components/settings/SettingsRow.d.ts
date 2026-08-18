import type { ReactNode } from 'react';
export interface SettingsRowProps {
    label: ReactNode;
    description?: ReactNode;
    /** The control rendered at the trailing edge of the row. */
    control?: ReactNode;
    /** Children are an alias for the control slot. */
    children?: ReactNode;
    className?: string;
}
export declare function SettingsRow({ label, description, control, children, className }: SettingsRowProps): import("react").JSX.Element;
