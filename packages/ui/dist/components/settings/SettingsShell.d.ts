import type { ReactNode } from 'react';
import { type SettingsNavEntry, type SettingsNavItemName } from './SettingsNav';
export type SettingsSectionName = SettingsNavItemName;
export interface SettingsShellProps {
    /** Selected sidebar page. Defaults to `Profiles`. */
    activeSection?: string;
    /** Aliases for `activeSection`, useful when embedding the shell in a router. */
    activePage?: string;
    selectedSection?: string;
    /** Called when the sidebar selection changes. */
    onSectionChange?: (section: string) => void;
    /** Aliases for `onSectionChange`. */
    onPageChange?: (section: string) => void;
    onNavigate?: (section: string) => void;
    /** Override the built-in eight-item navigation. */
    navItems?: readonly SettingsNavEntry[];
    /** Optional content rendered alongside the sidebar. */
    children?: ReactNode;
    /** Alias for `children`. */
    content?: ReactNode;
    className?: string;
    sidebarClassName?: string;
    contentClassName?: string;
}
export declare function SettingsShell({ activeSection, activePage, selectedSection, onSectionChange, onPageChange, onNavigate, navItems, children, content, className, sidebarClassName, contentClassName, }: SettingsShellProps): import("react").JSX.Element;
