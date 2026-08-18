import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import styles from './settings.module.css';
export function SettingsSection({ label, title, description, children, actions, className, }) {
    const heading = label ?? title;
    const headingId = typeof heading === 'string' ? `settings-section-${heading.toLowerCase().replace(/[^a-z0-9]+/g, '-')}` : undefined;
    return (_jsxs("section", { className: cx(styles.section, className), "aria-labelledby": headingId, children: [(heading !== undefined || description !== undefined || actions !== undefined) && (_jsxs("header", { className: styles.sectionHeader, children: [_jsxs("div", { children: [heading !== undefined && (_jsx("h2", { className: styles.sectionLabel, id: headingId, children: heading })), description !== undefined && _jsx("p", { className: styles.sectionDescription, children: description })] }), actions !== undefined && _jsx("div", { className: styles.headerActions, children: actions })] })), _jsx("div", { className: styles.sectionBody, children: children })] }));
}
