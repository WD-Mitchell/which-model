import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { usePointerFraction } from '../../hooks/usePointerFraction';
const MUTED = 'color-mix(in srgb,var(--color-text) 72%,transparent)';
const DIM = 'color-mix(in srgb,var(--color-text) 45%,transparent)';
const UNFILLED = 'color-mix(in srgb,var(--color-text) 12%,transparent)';
const KNOB_SHADOW = '0 0 0 1.5px var(--color-accent)';
/** Display clamp: [0, 5]. Kept open at the bottom so a 0 that already exists
 *  in a config still reads as "ignored" instead of being shown as a 1. */
function clampWeight(value) {
    if (!Number.isFinite(value))
        return 0;
    return Math.max(0, Math.min(5, Math.round(value)));
}
/** Interaction clamp: [min, 5]. What a drag or an arrow key may produce. */
function clampInput(value, min) {
    if (!Number.isFinite(value))
        return min;
    return Math.max(min, Math.min(5, Math.round(value)));
}
/**
 * Where a value sits along the track, as a percentage.
 *
 * The scale runs from `min` to 5, so a 1..5 row puts 1 at the far LEFT and 5
 * at the far right — the whole track is the range the row can actually take.
 * (A 0..5 row is unchanged: 0 is empty, 5 is full.) Without this, a 1..5 row
 * parked its floor a fifth of the way in and the leftmost stretch of track was
 * unreachable dead zone.
 */
function weightPercentage(value, min = 0) {
    const span = 5 - min;
    return `${((value - min) / span) * 100}%`;
}
function StepTrack({ value, min }) {
    return (_jsx("span", { "data-testid": "weight-step-track", style: { display: 'flex', gap: '3px', width: '100%' }, children: (min === 1 ? [2, 3, 4, 5] : [1, 2, 3, 4, 5]).map((step) => (_jsx("span", { "data-testid": "weight-step", style: {
                flex: 1,
                height: '6px',
                borderRadius: '2px',
                background: step <= value ? 'var(--color-accent-500)' : UNFILLED,
            } }, step))) }));
}
function BarTrack({ value, min }) {
    const width = weightPercentage(value, min);
    return (_jsx("span", { "data-testid": "weight-bar-track", style: {
            display: 'block',
            position: 'relative',
            width: '100%',
            height: '6px',
            borderRadius: '3px',
            background: UNFILLED,
            overflow: 'hidden',
        }, children: _jsx("b", { "data-testid": "weight-bar-fill", style: {
                display: 'block',
                position: 'absolute',
                inset: '0 auto 0 0',
                width,
                borderRadius: '3px',
                background: 'var(--color-accent-500)',
            } }) }));
}
function SliderTrack({ value, min }) {
    const width = weightPercentage(value, min);
    return (_jsxs("span", { "data-testid": "weight-slider-track", style: {
            display: 'block',
            position: 'relative',
            width: '100%',
            height: '3px',
            borderRadius: '2px',
            background: UNFILLED,
        }, children: [_jsx("b", { "data-testid": "weight-slider-fill", style: {
                    display: 'block',
                    position: 'absolute',
                    inset: '0 auto 0 0',
                    width,
                    borderRadius: '2px',
                    background: 'var(--color-accent-500)',
                } }), _jsx("i", { "data-testid": "weight-slider-knob", style: {
                    display: 'block',
                    position: 'absolute',
                    top: '-4.5px',
                    left: `calc(${width} - 6px)`,
                    width: '12px',
                    height: '12px',
                    borderRadius: '50%',
                    background: 'var(--color-bg)',
                    boxShadow: KNOB_SHADOW,
                } })] }));
}
export function WeightRow({ variant, label, value, accent = false, readOnly = false, labelWidth = 104, valueStyle = 'compact', min = 0, onChange, onRemove, }) {
    const current = clampWeight(value);
    const isVerbose = valueStyle === 'verbose';
    const labelColor = accent ? 'var(--color-accent-300)' : current > 0 ? MUTED : DIM;
    const controlCursor = readOnly ? 'default' : 'pointer';
    // The pointer maps onto [min, 5] rather than [0, 5], so the track's left end
    // IS the floor: a 1..5 row reaches 1 at 0% and 5 at 100%, with the five
    // values evenly spread between them.
    const onFraction = usePointerFraction((fraction) => {
        if (readOnly)
            return;
        const next = clampInput(min + Math.round(fraction * (5 - min)), min);
        if (next !== current)
            onChange?.(next);
    });
    const onKeyDown = (event) => {
        if (readOnly)
            return;
        let delta = 0;
        if (event.key === 'ArrowRight' || event.key === 'ArrowUp')
            delta = 1;
        if (event.key === 'ArrowLeft' || event.key === 'ArrowDown')
            delta = -1;
        if (delta === 0)
            return;
        event.preventDefault();
        const next = clampInput(current + delta, min);
        if (next !== current)
            onChange?.(next);
    };
    const rowStyle = {
        display: 'flex',
        alignItems: 'center',
        gap: isVerbose ? '12px' : '10px',
        fontSize: '12.5px',
        minWidth: 0,
    };
    const labelStyle = {
        flex: `0 0 ${labelWidth}px`,
        width: `${labelWidth}px`,
        minWidth: 0,
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        color: labelColor,
        fontFamily: 'var(--font-mono)',
        fontSize: '11.5px',
    };
    const controlStyle = {
        flex: '1 1 auto',
        minWidth: 0,
        maxWidth: isVerbose ? '300px' : undefined,
        position: 'relative',
        padding: '5px 0',
        cursor: controlCursor,
        outline: 'none',
    };
    const valueStyleObject = {
        flex: `0 0 ${isVerbose ? '56px' : '12px'}`,
        width: isVerbose ? '56px' : '12px',
        minWidth: isVerbose ? '56px' : '12px',
        textAlign: 'right',
        color: isVerbose && current === 0 ? DIM : accent ? 'var(--color-accent-300)' : current > 0 ? MUTED : DIM,
        fontFamily: 'var(--font-mono)',
        fontSize: isVerbose ? '11.5px' : '11px',
        whiteSpace: 'nowrap',
    };
    return (_jsxs("div", { "data-testid": "weight-row", style: rowStyle, children: [_jsx("span", { style: labelStyle, title: label, children: label }), _jsxs("span", { "data-testid": "weight-control", role: "slider", "aria-label": label, "aria-valuemin": min, "aria-valuemax": 5, "aria-valuenow": current, tabIndex: readOnly ? undefined : 0, onPointerDown: readOnly ? undefined : onFraction, onKeyDown: readOnly ? undefined : onKeyDown, style: controlStyle, children: [variant === 'step' ? _jsx(StepTrack, { value: current, min: min }) : null, variant === 'bar' ? _jsx(BarTrack, { value: current, min: min }) : null, variant === 'slider' ? _jsx(SliderTrack, { value: current, min: min }) : null] }), _jsx("span", { style: valueStyleObject, children: isVerbose ? (current > 0 ? `${current} / 5` : 'ignored') : current }), isVerbose ? null : onRemove && !readOnly ? (_jsx("button", { type: "button", className: "ib", "aria-label": `Remove ${label}`, onClick: onRemove, style: {
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flex: '0 0 14px',
                    width: '14px',
                    height: '14px',
                    padding: 0,
                    border: 0,
                    color: 'color-mix(in srgb,var(--color-text) 40%,transparent)',
                    background: 'transparent',
                    cursor: 'pointer',
                }, children: _jsx("svg", { width: "9", height: "9", viewBox: "0 0 12 12", fill: "none", "aria-hidden": "true", children: _jsx("path", { d: "M3 3l6 6M9 3l-6 6", stroke: "currentColor", strokeWidth: "1.7", strokeLinecap: "round" }) }) })) : (_jsx("span", { "aria-hidden": "true", style: { flex: '0 0 14px', width: '14px' } }))] }));
}
