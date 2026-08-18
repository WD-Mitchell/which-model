import { useEffect, useRef } from 'react'
import type React from 'react'

/**
 * Replicates the mockup `drag()` helper. The returned handler is attached as
 * `onPointerDown`; it captures the currentTarget rect, registers window
 * pointermove/pointerup listeners, calls `onFraction(clamp((clientX-left)/width, 0, 1))`
 * immediately on the down event and on every move, removes both listeners on
 * pointerup (and on unmount), and calls preventDefault + stopPropagation on the
 * down event.
 */
export function usePointerFraction(
  onFraction: (f: number) => void,
): (e: React.PointerEvent<HTMLElement>) => void {
  const onFractionRef = useRef(onFraction)
  onFractionRef.current = onFraction
  const cleanupRef = useRef<() => void>(() => {})
  useEffect(() => () => cleanupRef.current(), [])

  return (e: React.PointerEvent<HTMLElement>) => {
    const el = e.currentTarget
    const r = el.getBoundingClientRect()
    const at = (ev: PointerEvent) =>
      onFractionRef.current(Math.min(1, Math.max(0, (ev.clientX - r.left) / r.width)))
    const up = () => {
      window.removeEventListener('pointermove', at)
      window.removeEventListener('pointerup', up)
      cleanupRef.current = () => {}
    }
    cleanupRef.current = up
    window.addEventListener('pointermove', at)
    window.addEventListener('pointerup', up)
    at(e as unknown as PointerEvent)
    e.preventDefault()
    e.stopPropagation()
  }
}