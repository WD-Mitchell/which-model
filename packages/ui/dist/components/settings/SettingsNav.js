import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import styles from './settings.module.css';
/** The settings pages exposed by the desktop settings window. */
export const SETTINGS_NAV_ITEMS = [
    'Profiles',
    'Groups & benchmarks',
    'Providers',
    'Harnesses',
    'General',
    'Usage',
    'Favourites',
    'Agent hooks',
];
function entryDetails(entry, index) {
    if (typeof entry === 'string') {
        return { id: entry, label: entry, disabled: false };
    }
    const label = entry.label ?? entry.name ?? entry.id ?? entry.key ?? entry.value ?? `Item ${index + 1}`;
    const id = entry.id ?? entry.key ?? entry.value ?? (typeof label === 'string' ? label : `item-${index}`);
    return { id, label, disabled: entry.disabled ?? false };
}
export function SettingsNav({ items = SETTINGS_NAV_ITEMS, activeItem, active, value, onSelect, onItemSelect, onChange, className, 'aria-label': ariaLabel = 'Settings navigation', }) {
    const selected = activeItem ?? active ?? value ?? entryDetails(items[0] ?? '', 0).id;
    const select = onSelect ?? onItemSelect ?? onChange;
    return (_jsx("nav", { className: cx(styles.nav, className), "aria-label": ariaLabel, children: items.map((entry, index) => {
            const item = entryDetails(entry, index);
            const isActive = item.id === selected;
            return (_jsxs("button", { type: "button", className: cx(styles.navItem, isActive && styles.navItemActive), "aria-current": isActive ? 'page' : undefined, "aria-pressed": isActive, disabled: item.disabled, onClick: () => select?.(item.id), children: [_jsx("span", { className: styles.navIndicator, "aria-hidden": "true" }), _jsx("span", { className: styles.navLabel, children: item.label })] }, item.id));
        }) }));
}
