// S04 — host switch (S00 CONTRACTS §4). getHost() returns the Wails-bound
// host in the webview and the in-memory MockEngineHost when running Vite in
// browser mode (`vite --mode browser`, i.e. `pnpm dev:browser`). The result
// is memoised (one host instance per page).
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { createWailsHost } from './wailsHost'

let cached: EngineHost | null = null

export function getHost(): EngineHost {
  if (cached) return cached
  const mock = import.meta.env.MODE === 'browser' || import.meta.env.MODE === 'test'
  cached = mock ? createMockEngineHost() : createWailsHost()
  return cached
}

/** Reset the memoised host (tests call this). Pass a custom host to inject. */
export function resetHost(next?: EngineHost): void {
  cached = next ?? null
}

export { createWailsHost } from './wailsHost'