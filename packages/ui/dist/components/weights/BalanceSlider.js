import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { usePointerFraction } from '../../hooks/usePointerFraction';
const KNOB_SHADOW = '0 0 0 1.5px var(--color-accent)';
function clampCore(value) {
    if (!Number.isFinite(value))
        return 10;
    return Math.max(10, Math.min(90, Math.round(value / 5) * 5));
}
export function BalanceSlider({ core, readOnly = false, showRatio = false, onChange }) {
    const current = clampCore(core);
    const task = 100 - current;
    const onFraction = usePointerFraction((fraction) => {
        if (readOnly)
            return;
        const next = Math.max(10, Math.min(90, Math.round(fraction * 20) * 5));
        if (next !== current)
            onChange?.(next);
    });
    const onKeyDown = (event) => {
        if (readOnly)
            return;
        let delta = 0;
        if (event.key === 'ArrowRight' || event.key === 'ArrowUp')
            delta = 5;
        if (event.key === 'ArrowLeft' || event.key === 'ArrowDown')
            delta = -5;
        if (delta === 0)
            return;
        event.preventDefault();
        const next = clampCore(current + delta);
        if (next !== current)
            onChange?.(next);
    };
    const captionStyle = {
        display: 'flex',
        alignItems: 'center',
        position: 'relative',
        fontFamily: 'var(--font-mono)',
        fontSize: '10px',
        letterSpacing: '.06em',
        textTransform: 'uppercase',
        color: 'color-mix(in srgb,var(--color-text) 55%,transparent)',
    };
    const sliderStyle = {
        display: 'flex',
        alignItems: 'center',
        gap: '3px',
        width: '100%',
        height: '14px',
        cursor: readOnly ? 'default' : 'pointer',
        outline: 'none',
    };
    return (_jsxs("div", { "data-testid": "balance-slider", style: { display: 'flex', flexDirection: 'column', gap: '7px' }, children: [_jsxs("div", { style: captionStyle, children: [_jsx("span", { children: "core" }), showRatio ? (_jsx("span", { style: {
                            position: 'absolute',
                            left: '50%',
                            transform: 'translateX(-50%)',
                            color: 'var(--color-accent-300)',
                        }, children: `${current} / ${task}` })) : null, _jsx("span", { style: { marginLeft: 'auto' }, children: "task" })] }), _jsxs("div", { "data-testid": "balance-control", role: "slider", "aria-label": "Core and task balance", "aria-valuemin": 10, "aria-valuemax": 90, "aria-valuenow": current, tabIndex: readOnly ? undefined : 0, onPointerDown: readOnly ? undefined : onFraction, onKeyDown: readOnly ? undefined : onKeyDown, style: sliderStyle, children: [_jsx("span", { "data-testid": "balance-core-bar", style: {
                            flex: current,
                            minWidth: 0,
                            height: '5px',
                            borderRadius: '3px',
                            background: 'var(--color-accent-500)',
                        } }), _jsx("span", { "data-testid": "balance-knob", "aria-hidden": "true", style: {
                            flex: '0 0 14px',
                            width: '14px',
                            height: '14px',
                            borderRadius: '50%',
                            background: 'var(--color-bg)',
                            boxShadow: KNOB_SHADOW,
                        } }), _jsx("span", { "data-testid": "balance-task-bar", style: {
                            flex: task,
                            minWidth: 0,
                            height: '5px',
                            borderRadius: '3px',
                            background: 'var(--color-accent-800)',
                        } })] })] }));
}
