import type { ReactNode } from 'react';
export interface SettingsDetailShellProps {
    /** Content for the master/list pane, when the parent uses a split view. */
    master?: ReactNode;
    /** Content for the detail pane. Children are used when omitted. */
    detail?: ReactNode;
    children?: ReactNode;
    title?: ReactNode;
    description?: ReactNode;
    /** The back button is rendered when a callback is supplied. */
    onBack?: () => void;
    backLabel?: string;
    actions?: ReactNode;
    className?: string;
    bodyClassName?: string;
}
export declare function SettingsDetailShell({ master, detail, children, title, description, onBack, backLabel, actions, className, bodyClassName, }: SettingsDetailShellProps): import("react").JSX.Element;
