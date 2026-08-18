// U05 — app-level host access. Delegates to the S04 host switch so the
// browser-mode mock (vite --mode browser) and the Wails webview host (plain
// vite) both flow through one getHost(). window.* shims stay here so the
// components call these module functions (never the host directly) for
// mutation/side-effect effects inside lib boundaries.
import type { MockData } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { getHost as switchGetHost, resetHost as switchResetHost } from '../host'

export type MockEngineHost = EngineHost & { data: MockData }

export function getHost(): EngineHost {
  return switchGetHost()
}

/** Replace/reset the singleton host. Tests call `resetHost()` (or pass a
 *  custom host) in `beforeEach` for an isolated MockEngineHost per case. */
export function resetHost(next?: EngineHost): void {
  switchResetHost(next)
}

export function openSettings(): Promise<void> {
  return getHost().window.openSettings()
}

export function closeSettings(): Promise<void> {
  return getHost().window.closeSettings()
}

export function hidePopover(): Promise<void> {
  return getHost().window.hidePopover()
}

export function quit(): Promise<void> {
  return getHost().window.quit()
}

export function copyToClipboard(text: string): Promise<void> {
  return getHost().window.copyToClipboard(text)
}