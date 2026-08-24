// Tray → popover channel (S02 SPEC §2.6). The menu-bar right-click menu lets
// the user quick-select a profile; the Go host records the choice, shows the
// popover, and emits "tray:profile" with {slug} (cmd/which-model-desktop/
// traymenu.go). This module is the only place the frontend listens for it.
//
// "tray:profile" is deliberately NOT a member of the closed EngineEvent enum in
// packages/core: no engine facet emits it, it is host↔app plumbing, and the
// core mirror must keep describing only D00 §3 engine events. That is why this
// subscribes through the Wails runtime directly rather than through
// EngineHost.on().
import { Events } from '@wailsio/runtime'

/** Event names, matching traymenu.go's trayProfileEvent / trayViewEvent. */
const TRAY_PROFILE_EVENT = 'tray:profile'
const TRAY_VIEW_EVENT = 'tray:view'

/** True outside the Wails webview: `pnpm dev:browser` and vitest. */
const mockMode = (): boolean =>
  import.meta.env.MODE === 'browser' || import.meta.env.MODE === 'test'

/**
 * Outside the Wails runtime these channels fall back to a DOM CustomEvent of
 * the same name, so browser-mode development and tests can still drive them:
 *
 *   window.dispatchEvent(new CustomEvent('tray:view', { detail: { view: 'weights' } }))
 *
 * This matters beyond testing — the weights view is reachable only from the
 * tray menu now, so without a fallback it would be unreachable in `dev:browser`.
 */
function onMockEvent(name: string, cb: (detail: unknown) => void): () => void {
  const handler = (e: Event) => cb((e as CustomEvent).detail)
  window.addEventListener(name, handler)
  return () => window.removeEventListener(name, handler)
}

/**
 * Subscribe to tray profile quick-selects. Returns the unsubscribe function.
 *
 * In browser (`pnpm dev:browser`) and test modes there is no Wails runtime, so
 * this is inert and returns a no-op disposer — the same switch `src/host` uses
 * to pick the mock host.
 */
export function onTrayProfile(cb: (slug: string) => void): () => void {
  if (mockMode()) {
    return onMockEvent(TRAY_PROFILE_EVENT, (detail) => {
      const slug = readField(detail, 'slug')
      if (slug) cb(slug)
    })
  }
  return Events.On(TRAY_PROFILE_EVENT, (ev) => {
    const slug = readSlug(ev?.data)
    if (slug) cb(slug)
  })
}

/**
 * Pull the slug out of the event payload. Wails delivers a single emitted value
 * as `data` and multiple values as an array, so both shapes are accepted; an
 * unrecognised payload yields '' and is ignored rather than throwing inside a
 * runtime callback.
 */
function readSlug(data: unknown): string {
  const payload = Array.isArray(data) ? data[0] : data
  if (payload && typeof payload === 'object') {
    const slug = (payload as { slug?: unknown }).slug
    if (typeof slug === 'string') return slug
  }
  return ''
}

/**
 * Subscribe to tray view requests — the menu's "Custom weights…", which the
 * popover's own hamburger used to own before that menu moved to the tray.
 * Returns the unsubscribe function; inert outside the Wails runtime.
 */
export function onTrayView(cb: (view: string) => void): () => void {
  if (mockMode()) {
    return onMockEvent(TRAY_VIEW_EVENT, (detail) => {
      const view = readField(detail, 'view')
      if (view) cb(view)
    })
  }
  return Events.On(TRAY_VIEW_EVENT, (ev) => {
    const view = readField(ev?.data, 'view')
    if (view) cb(view)
  })
}

/** Generic form of readSlug for the other single-field payloads. */
function readField(data: unknown, key: string): string {
  const payload = Array.isArray(data) ? data[0] : data
  if (payload && typeof payload === 'object') {
    const value = (payload as Record<string, unknown>)[key]
    if (typeof value === 'string') return value
  }
  return ''
}
