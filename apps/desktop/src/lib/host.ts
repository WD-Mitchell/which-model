import { createMockEngineHost } from '@which-model/core/mock'
import type { MockData } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'

/** The concrete mock host (EngineHost + the mutable `data` fixture), for
 *  tests that need to inspect/seed state via `getHost() as MockEngineHost`. */
export type MockEngineHost = EngineHost & { data: MockData }

// U05 — app-level host access. The real Tauri-bound host is wired by S04;
// until then every app entry runs against the in-memory MockEngineHost so
// the full surface is exercisable in the browser.
let host: EngineHost | null = null

export function getHost(): EngineHost {
  if (!host) host = createMockEngineHost()
  return host
}

/** Replace/reset the singleton host. Tests call `resetHost()` (or pass a
 *  custom host) in `beforeEach` for an isolated MockEngineHost per case. */
export function resetHost(next?: EngineHost): void {
  host = next ?? null
}

// window.* shims — safe no-ops on the mock host in browser mode; the Tauri
// backend backs the same methods via S04. Components call these (never the
// host directly) so mutation/minor effects stay behind lib boundaries.
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