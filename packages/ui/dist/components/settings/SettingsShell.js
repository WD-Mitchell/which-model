import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import { SETTINGS_NAV_ITEMS, SettingsNav } from './SettingsNav';
import styles from './settings.module.css';
export function SettingsShell({ activeSection, activePage, selectedSection, onSectionChange, onPageChange, onNavigate, navItems = SETTINGS_NAV_ITEMS, children, content, className, sidebarClassName, contentClassName, }) {
    const selected = activeSection ?? activePage ?? selectedSection;
    const onChange = onSectionChange ?? onPageChange ?? onNavigate;
    return (_jsxs("div", { className: cx(styles.shell, className), "data-testid": "settings-shell", children: [_jsx("aside", { className: cx(styles.sidebar, sidebarClassName), children: _jsx(SettingsNav, { items: navItems, activeItem: selected, onSelect: onChange }) }), _jsx("main", { className: cx(styles.content, contentClassName), children: content ?? children })] }));
}
