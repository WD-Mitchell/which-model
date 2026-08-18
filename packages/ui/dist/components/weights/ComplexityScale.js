import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { usePointerFraction } from '../../hooks/usePointerFraction';
const KNOB_SHADOW = '0 0 0 1.5px var(--color-accent)';
function clampStop(value) {
    if (!Number.isFinite(value))
        return 0;
    return Math.max(0, Math.min(4, Math.round(value)));
}
export function ComplexityScale({ stop, labels = ['simple action', 'planning'], profileName, readOnly = false, onStop, }) {
    const current = clampStop(stop);
    const onFraction = usePointerFraction((fraction) => {
        if (readOnly)
            return;
        const next = clampStop(Math.round(fraction * 4));
        if (next !== current)
            onStop?.(next);
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
        const next = clampStop(current + delta);
        if (next !== current)
            onStop?.(next);
    };
    const controlStyle = {
        position: 'relative',
        width: '100%',
        height: '14px',
        cursor: readOnly ? 'default' : 'pointer',
        outline: 'none',
    };
    const trackStyle = {
        position: 'absolute',
        top: '5px',
        left: 0,
        right: 0,
        height: '5px',
        borderRadius: '3px',
        background: 'linear-gradient(to right, var(--color-accent-800), var(--color-accent-500))',
    };
    return (_jsxs("div", { "data-testid": "complexity-scale", children: [_jsx("div", { "data-testid": "complexity-control", role: "slider", "aria-label": profileName ? `${profileName} complexity` : 'Complexity', "aria-valuemin": 0, "aria-valuemax": 4, "aria-valuenow": current, tabIndex: readOnly ? undefined : 0, onPointerDown: readOnly ? undefined : onFraction, onKeyDown: readOnly ? undefined : onKeyDown, style: controlStyle, children: _jsxs("span", { "data-testid": "complexity-track", style: trackStyle, children: [[0, 1, 2, 3, 4].map((tick) => (_jsx("span", { "data-testid": "complexity-tick", style: {
                                position: 'absolute',
                                top: '-4px',
                                left: `${tick * 25}%`,
                                width: '1px',
                                height: '13px',
                                background: 'color-mix(in srgb,var(--color-text) 20%,transparent)',
                            } }, tick))), _jsx("i", { "data-testid": "complexity-knob", "aria-hidden": "true", style: {
                                position: 'absolute',
                                top: '-5px',
                                left: `calc(${current * 25}% - 7px)`,
                                width: '15px',
                                height: '15px',
                                borderRadius: '50%',
                                background: 'var(--color-bg)',
                                boxShadow: KNOB_SHADOW,
                            } })] }) }), _jsxs("div", { style: {
                    display: 'flex',
                    justifyContent: 'space-between',
                    marginTop: '9px',
                    fontFamily: 'var(--font-mono)',
                    fontSize: '9px',
                    color: 'color-mix(in srgb,var(--color-text) 45%,transparent)',
                }, children: [_jsx("span", { children: labels[0] }), _jsx("span", { children: labels[1] })] }), profileName ? (_jsx("div", { style: {
                    marginTop: '15px',
                    textAlign: 'center',
                    fontFamily: 'var(--font-heading)',
                    fontSize: '15px',
                    color: 'var(--color-accent-300)',
                }, children: profileName })) : null] }));
}
