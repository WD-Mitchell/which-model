import { jsx as _jsx, jsxs as _jsxs, Fragment as _Fragment } from "react/jsx-runtime";
import { WeightRow } from './WeightRow';
const HEADER_COLOR = 'color-mix(in srgb,var(--color-text) 42%,transparent)';
const PCT_COLOR = 'color-mix(in srgb,var(--color-text) 62%,transparent)';
const POPUP_TEXT = 'color-mix(in srgb,var(--color-text) 80%,transparent)';
function SectionHeader({ children, percentage, }) {
    return (_jsxs("div", { style: { display: 'flex', alignItems: 'baseline', gap: '8px' }, children: [_jsx("span", { style: {
                    color: HEADER_COLOR,
                    fontSize: '10px',
                    letterSpacing: '.11em',
                    textTransform: 'uppercase',
                }, children: children }), _jsx("span", { style: {
                    marginLeft: 'auto',
                    color: PCT_COLOR,
                    fontFamily: 'var(--font-mono)',
                    fontSize: '10.5px',
                }, children: percentage })] }));
}
function Rows({ rows, sliderStyle, labelWidth, valueStyle, readOnly, removable, onChangeWeight, onRemoveWeight, }) {
    return (_jsx(_Fragment, { children: rows.map((row) => (_jsx(WeightRow, { variant: sliderStyle, label: row.key, value: row.value, accent: row.accent, labelWidth: labelWidth, valueStyle: valueStyle, readOnly: readOnly, 
            // 1..5, never 0: a metric is dropped with the row's × button, and a
            // 0 would be a weight the engine refuses to rank on.
            min: 1, onChange: onChangeWeight ? (value) => onChangeWeight(row.key, value) : undefined, onRemove: removable && !readOnly && onRemoveWeight ? () => onRemoveWeight(row.key) : undefined }, row.key))) }));
}
export function WeightEditor({ variant, sliderStyle, coreRows, taskRows, sectionPcts, readOnly = false, addable = [], addOpen = false, onChangeWeight, onRemoveWeight, onAddMetric, onToggleAdd, onRevert, extraActions, }) {
    const isPopover = variant === 'popover';
    const labelWidth = isPopover ? 104 : 150;
    const valueStyle = isPopover ? 'compact' : 'verbose';
    const actionRowStyle = {
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        position: 'relative',
        padding: '2px 0 10px',
    };
    const buttonStyle = {
        fontSize: '11.5px',
        padding: '2px 6px',
    };
    return (_jsxs("div", { "data-testid": "weight-editor", style: {
            display: 'flex',
            flexDirection: 'column',
            gap: '12px',
        }, children: [_jsxs(SectionHeader, { percentage: sectionPcts.core, children: ["core benchmarks", ' ', _jsx("span", { style: { textTransform: 'none', letterSpacing: 0 }, children: "(higher = better, cheaper, faster)" })] }), _jsx(Rows, { rows: coreRows, sliderStyle: sliderStyle, labelWidth: labelWidth, valueStyle: valueStyle, readOnly: readOnly, removable: false, onChangeWeight: onChangeWeight }), _jsx(SectionHeader, { percentage: sectionPcts.task, children: "task benchmarks" }), _jsx(Rows, { rows: taskRows, sliderStyle: sliderStyle, labelWidth: labelWidth, valueStyle: valueStyle, readOnly: readOnly, removable: isPopover, onChangeWeight: onChangeWeight, onRemoveWeight: onRemoveWeight }), isPopover ? (_jsxs("div", { style: actionRowStyle, children: [_jsx("button", { type: "button", className: "btn btn-ghost", style: buttonStyle, onClick: onToggleAdd, children: "+ Add metric" }), _jsxs("span", { style: { marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 4 }, children: [extraActions, _jsx("button", { type: "button", className: "btn btn-ghost", style: buttonStyle, onClick: onRevert, children: "Revert" })] }), addOpen ? (_jsx("span", { "data-testid": "weight-editor-add-popup", style: {
                            position: 'absolute',
                            left: 0,
                            bottom: '26px',
                            zIndex: 9,
                            width: '180px',
                            maxHeight: '150px',
                            padding: '5px',
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '1px',
                            overflow: 'auto',
                            borderRadius: '8px',
                            background: 'var(--color-surface)',
                            boxShadow: 'var(--shadow-md)',
                        }, children: addable.map((key) => (_jsx("span", { role: "button", tabIndex: 0, onClick: () => onAddMetric?.(key), onKeyDown: (event) => {
                                if (event.key === 'Enter' || event.key === ' ') {
                                    event.preventDefault();
                                    onAddMetric?.(key);
                                }
                            }, style: {
                                padding: '6px 9px',
                                borderRadius: '5px',
                                color: POPUP_TEXT,
                                fontFamily: 'var(--font-mono)',
                                fontSize: '11.5px',
                                cursor: 'pointer',
                            }, children: key }, key))) })) : null] })) : null] }));
}
