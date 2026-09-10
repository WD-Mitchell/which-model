import { vi } from 'vitest'

// Every desktop test can import Wails through the host switch. Own its startup
// timers here so the drag polling interval cannot outlive the jsdom window.
// Keep the real runtime for binding/error tests; app tests use real timers.
vi.useFakeTimers()
try {
  await import('@wailsio/runtime')
  vi.runOnlyPendingTimers()
  if (vi.getTimerCount() !== 0) {
    throw new Error('Wails startup left pending timers before desktop tests')
  }
} finally {
  vi.useRealTimers()
}
