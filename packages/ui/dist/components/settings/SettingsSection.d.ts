import type { ReactNode } from 'react';
export interface SettingsSectionProps {
    /** Section heading. `label` is the preferred name in settings forms. */
    label?: ReactNode;
    title?: ReactNode;
    description?: ReactNode;
    children?: ReactNode;
    /** Optional content aligned with the section heading. */
    actions?: ReactNode;
    className?: string;
}
export declare function SettingsSection({ label, title, description, children, actions, className, }: SettingsSectionProps): import("react").JSX.Element;
