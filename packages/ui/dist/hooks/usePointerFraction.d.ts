import type React from 'react';
/**
 * Replicates the mockup `drag()` helper. The returned handler is attached as
 * `onPointerDown`; it captures the currentTarget rect, registers window
 * pointermove/pointerup listeners, calls `onFraction(clamp((clientX-left)/width, 0, 1))`
 * immediately on the down event and on every move, removes both listeners on
 * pointerup (and on unmount), and calls preventDefault + stopPropagation on the
 * down event.
 */
export declare function usePointerFraction(onFraction: (f: number) => void): (e: React.PointerEvent<HTMLElement>) => void;
