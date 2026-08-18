import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { usePointerFraction } from '../usePointerFraction'

function Rects() {
  return {
    left: 100,
    width: 200,
    top: 0,
    right: 300,
    bottom: 20,
    height: 20,
    x: 100,
    y: 0,
    toJSON: () => ({}),
  }
}

function Target({ onFraction }: { onFraction: (f: number) => void }) {
  const onPointerDown = usePointerFraction(onFraction)
  return (
    <div
      data-testid="target"
      onPointerDown={(e) => onPointerDown(e)}
      ref={(el) => {
        if (el) el.getBoundingClientRect = () => Rects() as DOMRect
      }}
    />
  )
}

describe('usePointerFraction', () => {
  it('fires onFraction on down, clamped on move, and stops after pointerup', () => {
    const onFraction = vi.fn()
    const { getByTestId } = render(<Target onFraction={onFraction} />)
    const el = getByTestId('target')

    fireEvent.pointerDown(el, { clientX: 150, clientY: 5 })
    // (150 - 100) / 200 = 0.25
    expect(onFraction).toHaveBeenLastCalledWith(0.25)

    fireEvent.pointerMove(window, { clientX: 400, clientY: 5 })
    // (400 - 100) / 200 = 1.5 → clamped to 1
    expect(onFraction).toHaveBeenLastCalledWith(1)

    const count = onFraction.mock.calls.length
    fireEvent.pointerUp(window)
    fireEvent.pointerMove(window, { clientX: 300, clientY: 5 })
    expect(onFraction.mock.calls.length).toBe(count)
  })

  it('calls preventDefault and stopPropagation on the down event', () => {
    const onFraction = vi.fn()
    const { getByTestId } = render(<Target onFraction={onFraction} />)
    const el = getByTestId('target')

    const onDoc = vi.fn()
    document.addEventListener('pointerdown', onDoc)
    const ev = new Event('pointerdown', { bubbles: true, cancelable: true })
    el.dispatchEvent(ev)
    document.removeEventListener('pointerdown', onDoc)

    expect(ev.defaultPrevented).toBe(true)
    expect(onDoc).not.toHaveBeenCalled()
  })

  it('removes listeners on unmount mid-drag', () => {
    const onFraction = vi.fn()
    const { getByTestId, unmount } = render(<Target onFraction={onFraction} />)
    const el = getByTestId('target')

    fireEvent.pointerDown(el, { clientX: 150, clientY: 5 })
    expect(onFraction).toHaveBeenCalledTimes(1)

    unmount()
    fireEvent.pointerMove(window, { clientX: 400, clientY: 5 })
    fireEvent.pointerUp(window)
    expect(onFraction).toHaveBeenCalledTimes(1)
  })
})