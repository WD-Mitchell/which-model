import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

afterEach(() => {
  cleanup()
})

// jsdom does not implement ResizeObserver, which @dnd-kit uses for measuring.
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe(): void {}
    unobserve(): void {}
    disconnect(): void {}
  }
}

// jsdom does not ship cores that DragList's PointerSensor path relies on when
// real PointerEvent instances are constructed; visible here for pointer tests.
if (typeof globalThis.PointerEvent === 'undefined') {
  globalThis.PointerEvent =
    globalThis.MouseEvent ??
    class PointerEvent extends Event {
      clientX = 0
      clientY = 0
      button = 0
      isPrimary = true
      pointerType = 'mouse'
      constructor(type: string, props: Record<string, unknown> = {}) {
        super(type, props)
        Object.assign(this, props)
      }
    }
}