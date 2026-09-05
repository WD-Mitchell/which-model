import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { cx } from '../../utils/cx';
import { Toggle } from '../../primitives/Toggle';
import { UsageMeter } from '../../primitives/UsageMeter';
import styles from './ProviderUsageRow.module.css';
/**
 * One provider as a usage card: switch, id over its auth line, the three usage
 * meters, then credits over the reset hint.
 *
 * Ported from the harness detail view (mockup demo.dc.html 542-566, bindings
 * 1096-1118) and shared, because the same row is the Providers page's list —
 * it carries the live quota picture that a plain id-and-count row cannot.
 */
export function ProviderUsageRow({ provider, on, onToggle, live, offLabel = 'not enabled', trailing, leading, }) {
    // Only the windows this provider actually reports get a meter.
    //
    // The mockup drew all three because its fixture reported all three; real
    // providers do not (claude: session+weekly, copilot: monthly), and rendering
    // permanent "—" columns spent the width the live meters need. At three
    // meters each cell is ~62px while the label "SESSION" alone needs 65, so the
    // labels truncated to "SESSI…"; at one or two they fit outright.
    const windows = live
        ? [
            ['session', provider.session],
            ['weekly', provider.weekly],
            ['monthly', provider.monthly],
        ].filter(([, v]) => v !== null)
        : [];
    return (_jsxs("div", { className: cx(styles.row, on && styles.rowOn), children: [_jsx("span", { onClick: (e) => e.stopPropagation(), children: _jsx(Toggle, { on: on, onToggle: onToggle }) }), leading ? _jsx("span", { className: styles.leading, children: leading }) : null, _jsxs("span", { className: styles.idCell, children: [_jsx("span", { className: cx('mono', styles.id, on && styles.idOn), title: provider.id, children: provider.id }), _jsx("span", { className: cx('mono', styles.auth, !live && styles.authOff), children: live ? provider.auth : offLabel })] }), _jsx("span", { className: styles.meters, children: windows.length > 0 ? (windows.map(([id, value]) => _jsx(UsageMeter, { label: id, percent: value }, id))) : (_jsx("span", { className: cx('mono', styles.noUsage), children: live ? 'no usage data' : '' })) }), _jsxs("span", { className: styles.creditsCell, children: [_jsx("span", { className: cx('mono', styles.credits, !live && styles.creditsOff), children: live ? provider.credits : '' }), _jsx("span", { className: cx('mono', styles.resets), children: live ? provider.resets : '' })] }), trailing] }));
}
