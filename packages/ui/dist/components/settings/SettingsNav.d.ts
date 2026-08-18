import type { ReactNode } from 'react';
/** The settings pages exposed by the desktop settings window. */
export declare const SETTINGS_NAV_ITEMS: readonly ["Profiles", "Groups & benchmarks", "Providers", "Harnesses", "General", "Usage", "Favourites", "Agent hooks"];
export type SettingsNavItemName = (typeof SETTINGS_NAV_ITEMS)[number];
export interface SettingsNavItem {
    /** Stable value passed to the selection callback. */
    id?: string;
    /** `key` and `value` are accepted as aliases for `id` when composing a custom nav. */
    key?: string;
    value?: string;
    /** Text (or a custom node) shown in the sidebar. */
    label?: ReactNode;
    /** `name` is accepted as an alias for `label`. */
    name?: ReactNode;
    disabled?: boolean;
}
export type SettingsNavEntry = SettingsNavItem | string;
export interface SettingsNavProps {
    /** Custom entries; omitted entries use the eight settings pages. */
    items?: readonly SettingsNavEntry[];
    /** Selected entry id. Defaults to the first entry. */
    activeItem?: string;
    /** Alias for `activeItem`. */
    active?: string;
    /** Alias for `activeItem`. */
    value?: string;
    /** Called with the selected entry id. */
    onSelect?: (item: string) => void;
    /** Alias for `onSelect`. */
    onItemSelect?: (item: string) => void;
    /** Alias for `onSelect`. */
    onChange?: (item: string) => void;
    className?: string;
    'aria-label'?: string;
}
export declare function SettingsNav({ items, activeItem, active, value, onSelect, onItemSelect, onChange, className, 'aria-label': ariaLabel, }: SettingsNavProps): import("react").JSX.Element;
