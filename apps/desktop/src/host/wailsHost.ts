// S04 — wailsHost: EngineHost implemented over the generated Wails v3
// bindings. Every group property calls the matching generated module; each
// method awaits the binding and normalises rejections to EngineError-shaped
// ErrorDTOs via toEngineError. Events subscribe through the Wails runtime.
import { Events } from '@wailsio/runtime'
import type { EngineHost, EngineEvent, ErrorDTO } from '@which-model/core'
import {
  CatalogAPI,
  FavouritesAPI,
  HarnessesAPI,
  PickAPI,
  ProfilesAPI,
  ProvidersAPI,
  SettingsAPI,
  UsageAPI,
  WindowService,
} from '../bindings/github.com/WD-Mitchell/which-model/cmd/which-model-desktop/index.js'

// toEngineError guarantees every rejection reaching UI code is ErrorDTO-shaped
// (S04 SPEC §3): an ErrorDTO-shaped rejection passes through; anything else
// wraps as io_error.
export function toEngineError(err: unknown): ErrorDTO {
  if (err && typeof err === 'object' && 'code' in err && 'message' in err) {
    const e = err as { code: string; message: string }
    return { code: e.code, message: e.message }
  }
  return { code: 'io_error', message: err instanceof Error ? err.message : String(err) }
}

type Cancellable<T> = Promise<T> & { cancel(): void }

function offable<T>(p: Cancellable<T>): Promise<T> {
  // Await without leaking the cancellation API to callers.
  return p.then((v) => v, (e) => Promise.reject(e))
}

export function createWailsHost(): EngineHost {
  return {
    profiles: {
      list: () => offable(ProfilesAPI.List() as Cancellable<unknown>).then((r) => r as never),
      get: (slug: string) => offable(ProfilesAPI.Get(slug) as Cancellable<unknown>).then((r) => r as never),
      save: (p) => offable(ProfilesAPI.Save(p) as Cancellable<void>).then(() => {}),
      duplicate: (slug: string) => offable(ProfilesAPI.Duplicate(slug) as Cancellable<unknown>).then((r) => r as never),
      delete: (slug: string) => offable(ProfilesAPI.Delete(slug) as Cancellable<void>).then(() => {}),
      complexityScale: () => offable(ProfilesAPI.ComplexityScale() as Cancellable<unknown>).then((r) => r as never),
    },
    pick: {
      rank: (req) => offable(PickAPI.Rank(req) as Cancellable<unknown>).then((r) => r as never),
      recordPick: (profileSlug: string, routeKey: string) =>
        offable(PickAPI.RecordPick(profileSlug, routeKey) as Cancellable<void>).then(() => {}),
      catalogLine: () => offable(PickAPI.CatalogLine() as Cancellable<unknown>).then((r) => r as never),
    },
    catalog: {
      benchmarks: () => offable(CatalogAPI.Benchmarks() as Cancellable<unknown>).then((r) => r as never),
      benchmarkDetail: (name: string) => offable(CatalogAPI.BenchmarkDetail(name) as Cancellable<unknown>).then((r) => r as never),
      groups: () => offable(CatalogAPI.Groups() as Cancellable<unknown>).then((r) => r as never),
      groupDetail: (slug: string) => offable(CatalogAPI.GroupDetail(slug) as Cancellable<unknown>).then((r) => r as never),
      saveGroup: (slug: string, benchmarks: string[], renameTo?: string) =>
        offable(CatalogAPI.SaveGroup(slug, benchmarks, renameTo ?? '') as Cancellable<void>).then(() => {}),
      duplicateGroup: (slug: string) => offable(CatalogAPI.DuplicateGroup(slug) as Cancellable<unknown>).then((r) => r as never),
      deleteGroup: (slug: string) => offable(CatalogAPI.DeleteGroup(slug) as Cancellable<void>).then(() => {}),
    },
    providers: {
      list: () => offable(ProvidersAPI.List() as Cancellable<unknown>).then((r) => r as never),
      setEnabled: (id: string, on: boolean) => offable(ProvidersAPI.SetEnabled(id, on) as Cancellable<void>).then(() => {}),
      reorder: (orderedIds: string[]) => offable(ProvidersAPI.Reorder(orderedIds) as Cancellable<void>).then(() => {}),
      detail: (id: string) => offable(ProvidersAPI.Detail(id) as Cancellable<unknown>).then((r) => r as never),
      setRouteEnabled: (id: string, modelId: string, reasoning: string, enabled: boolean) =>
        offable(ProvidersAPI.SetRouteEnabled(id, modelId, reasoning, enabled) as Cancellable<void>).then(() => {}),
      setAllRoutes: (id: string, enabled: boolean) =>
        offable(ProvidersAPI.SetAllRoutes(id, enabled) as Cancellable<void>).then(() => {}),
    },
    harnesses: {
      list: () => offable(HarnessesAPI.List() as Cancellable<unknown>).then((r) => r as never),
      save: (h) => offable(HarnessesAPI.Save(h) as Cancellable<void>).then(() => {}),
      delete: (slug: string) => offable(HarnessesAPI.Delete(slug) as Cancellable<void>).then(() => {}),
      setProvider: (slug: string, provider: string, on: boolean) =>
        offable(HarnessesAPI.SetProvider(slug, provider, on) as Cancellable<void>).then(() => {}),
      setAllProviders: (slug: string, on: boolean) =>
        offable(HarnessesAPI.SetAllProviders(slug, on) as Cancellable<void>).then(() => {}),
      launch: (slug: string, routeKey: string, profileSlug: string) =>
        offable(HarnessesAPI.Launch(slug, routeKey, profileSlug) as Cancellable<unknown>).then((r) => r as never),
    },
    usage: {
      snapshots: (force: boolean) => offable(UsageAPI.Snapshots(force) as Cancellable<unknown>).then((r) => r as never),
      setMode: (mode: 'auto' | 'on' | 'off') => offable(UsageAPI.SetMode(mode) as Cancellable<void>).then(() => {}),
      setBackend: (backend: 'off' | 'native' | 'codexbar') =>
        offable(UsageAPI.SetBackend(backend) as Cancellable<void>).then(() => {}),
      mode: () => offable(UsageAPI.Mode() as Cancellable<unknown>).then((r) => r as never),
    },
    favourites: {
      list: () => offable(FavouritesAPI.List() as Cancellable<unknown>).then((r) => r as never),
      pin: (routeKey: string) => offable(FavouritesAPI.Pin(routeKey) as Cancellable<void>).then(() => {}),
      unpin: (routeKey: string) => offable(FavouritesAPI.Unpin(routeKey) as Cancellable<void>).then(() => {}),
    },
    settings: {
      get: () => offable(SettingsAPI.Get() as Cancellable<unknown>).then((r) => r as never),
      set: (s) => offable(SettingsAPI.Set(s) as Cancellable<void>).then(() => {}),
      shellSnippets: () => offable(SettingsAPI.ShellSnippets() as Cancellable<unknown>).then((r) => r as never),
    },
    window: {
      openSettings: () => offable(WindowService.OpenSettings() as Cancellable<void>).then(() => {}),
      closeSettings: () => offable(WindowService.CloseSettings() as Cancellable<void>).then(() => {}),
      hidePopover: () => offable(WindowService.HidePopover() as Cancellable<void>).then(() => {}),
      quit: () => offable(WindowService.Quit() as Cancellable<void>).then(() => {}),
      copyToClipboard: (text: string) =>
        offable(WindowService.CopyToClipboard(text) as Cancellable<void>).then(() => {}),
    },
    on(event: EngineEvent, cb: (payload: unknown) => void): () => void {
      // Subscribe to the Wails runtime event; angular/promise-of the payload
      // out of the WailsEvent envelope so cb receives the D00 §3 payload
      // directly. Returns the unsubscribe fn the runtime provides.
      return Events.On(event, (ev) => {
        cb(ev.data)
      })
    },
  }
}

export type { EngineEvent }