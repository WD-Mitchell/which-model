import type { ReactNode } from 'react';
export interface SettingsHeaderAction {
    label: string;
    onAction?: () => void;
    onClick?: () => void;
    disabled?: boolean;
}
export type SettingsHeaderActionSlot = ReactNode | SettingsHeaderAction;
export interface SettingsHeaderProps {
    title: ReactNode;
    /** Supporting copy below the title. */
    description?: ReactNode;
    /** `blurb` is an alias used by the settings page contracts. */
    blurb?: ReactNode;
    /** One or more already-rendered action buttons. */
    actions?: ReactNode;
    /** A convenience action object or a rendered action node. */
    action?: SettingsHeaderActionSlot;
    /** Convenience props for a single action button. */
    actionLabel?: string;
    onAction?: () => void;
    className?: string;
}
export declare function SettingsHeader({ title, description, blurb, actions, action, actionLabel, onAction, className, }: SettingsHeaderProps): import("react").JSX.Element;
