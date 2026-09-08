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
  SignInAPI,
  UsageAPI,
  WindowService,
} from '../bindings/github.com/WD-Mitchell/which-model/cmd/which-model-desktop/index.js'

// toEngineError guarantees every rejection reaching UI code is ErrorDTO-shaped
// (S04 SPEC §3): an ErrorDTO-shaped rejection passes through; anything else
// wraps as io_error.
const ERROR_CODES = new Set(['validation_failed', 'builtin_readonly', 'not_found', 'conflict', 'io_error', 'usage_unavailable', 'launch_failed'])

function isErrorDTO(value: unknown): value is ErrorDTO {
  return value !== null && typeof value === 'object' && !Array.isArray(value) &&
    'code' in value && typeof value.code === 'string' && ERROR_CODES.has(value.code) &&
    'message' in value && typeof value.message === 'string'
}

export function toEngineError(err: unknown): ErrorDTO {
  if (isErrorDTO(err)) return { code: err.code, message: err.message }
  if (err !== null && typeof err === 'object' && 'cause' in err && isErrorDTO(err.cause)) {
    return { code: err.cause.code, message: err.cause.message }
  }
  return { code: 'io_error', message: err instanceof Error ? err.message : String(err) }
}

type Cancellable<T> = Promise<T> & { cancel(): void }

function offable<T>(p: Cancellable<T>): Promise<T> {
  // Await without leaking the cancellation API to callers.
  return p.then((v) => v, (e) => Promise.reject(e))
}

// call wraps one binding invocation: rejections are normalised through
// toEngineError so every error reaching UI code is ErrorDTO-shaped
// (S04 CONTRACTS §4 — documented but unwired; issue #32). `then` maps the
// resolved value so callers keep the EngineHost's typed surface.
function call<T, R>(p: Cancellable<T>, then: (v: T) => R): Promise<R> {
  return offable(p).then(then, (e: unknown) => {
    throw toEngineError(e)
  })
}

export function createWailsHost(): EngineHost {
  return {
    profiles: {
      userProfiles: () => call(ProfilesAPI.UserProfiles() as Cancellable<unknown>, (r) => r as never),
      list: () => call(ProfilesAPI.List() as Cancellable<unknown>, (r) => r as never),
      get: (slug: string) => call(ProfilesAPI.Get(slug) as Cancellable<unknown>, (r) => r as never),
      create: (p) => call(ProfilesAPI.Create(p) as Cancellable<void>, () => {}),
      save: (p) => call(ProfilesAPI.Save(p) as Cancellable<void>, () => {}),
      duplicate: (slug: string) => call(ProfilesAPI.Duplicate(slug) as Cancellable<unknown>, (r) => r as never),
      delete: (slug: string) => call(ProfilesAPI.Delete(slug) as Cancellable<void>, () => {}),
      complexityScale: () => call(ProfilesAPI.ComplexityScale() as Cancellable<unknown>, (r) => r as never),
    },
    pick: {
      rank: (req) => call(PickAPI.Rank(req) as Cancellable<unknown>, (r) => r as never),
      recordPick: (profileSlug: string, routeKey: string) => call(PickAPI.RecordPick(profileSlug, routeKey) as Cancellable<void>, () => {}),
      catalogLine: () => call(PickAPI.CatalogLine() as Cancellable<unknown>, (r) => r as never),
    },
    catalog: {
      benchmarks: () => call(CatalogAPI.Benchmarks() as Cancellable<unknown>, (r) => r as never),
      benchmarkDetail: (name: string) => call(CatalogAPI.BenchmarkDetail(name) as Cancellable<unknown>, (r) => r as never),
      modelDetail: (model: string, reasoning: string) =>
        call(CatalogAPI.ModelDetail(model, reasoning) as Cancellable<unknown>, (r) => r as never),
      models: () => call(CatalogAPI.Models() as Cancellable<unknown>, (r) => r as never),
      model: (name: string) => call(CatalogAPI.Model(name) as Cancellable<unknown>, (r) => r as never),
      groups: () => call(CatalogAPI.Groups() as Cancellable<unknown>, (r) => r as never),
      groupDetail: (slug: string) => call(CatalogAPI.GroupDetail(slug) as Cancellable<unknown>, (r) => r as never),
      saveGroup: (slug: string, benchmarks: string[], renameTo?: string) =>
        call(CatalogAPI.SaveGroup(slug, benchmarks, renameTo ?? '') as Cancellable<void>, () => {}),
      duplicateGroup: (slug: string) => call(CatalogAPI.DuplicateGroup(slug) as Cancellable<unknown>, (r) => r as never),
      deleteGroup: (slug: string) => call(CatalogAPI.DeleteGroup(slug) as Cancellable<void>, () => {}),
    },
    providers: {
      add: (id: string) => call(ProvidersAPI.Add(id) as Cancellable<void>, () => {}),
      addable: () => call(ProvidersAPI.Addable() as Cancellable<unknown>, (r) => r as never),
      delete: (id: string) => call(ProvidersAPI.Delete(id) as Cancellable<void>, () => {}),
      duplicate: (id: string) => call(ProvidersAPI.Duplicate(id) as Cancellable<unknown>, (r) => r as never),
      setAccounts: (id, accounts) => call(ProvidersAPI.SetAccounts(id, accounts) as Cancellable<void>, () => {}),
      list: () => call(ProvidersAPI.List() as Cancellable<unknown>, (r) => r as never),
      setEnabled: (id: string, on: boolean) => call(ProvidersAPI.SetEnabled(id, on) as Cancellable<void>, () => {}),
      reorder: (orderedIds: string[]) => call(ProvidersAPI.Reorder(orderedIds) as Cancellable<void>, () => {}),
      detail: (id: string) => call(ProvidersAPI.Detail(id) as Cancellable<unknown>, (r) => r as never),
      setRouteEnabled: (id: string, modelId: string, reasoning: string, enabled: boolean) =>
        call(ProvidersAPI.SetRouteEnabled(id, modelId, reasoning, enabled) as Cancellable<void>, () => {}),
      setAllRoutes: (id: string, enabled: boolean) => call(ProvidersAPI.SetAllRoutes(id, enabled) as Cancellable<void>, () => {}),
      refreshRoutes: () => call(ProvidersAPI.RefreshRoutes() as Cancellable<void>, () => {}),
    },
    harnesses: {
      list: () => call(HarnessesAPI.List() as Cancellable<unknown>, (r) => r as never),
      save: (h) => call(HarnessesAPI.Save(h) as Cancellable<void>, () => {}),
      delete: (slug: string) => call(HarnessesAPI.Delete(slug) as Cancellable<void>, () => {}),
      setEnabled: (slug: string, enabled: boolean) => call(HarnessesAPI.SetEnabled(slug, enabled) as Cancellable<void>, () => {}),
      setProvider: (slug: string, provider: string, on: boolean) => call(HarnessesAPI.SetProvider(slug, provider, on) as Cancellable<void>, () => {}),
      setAllProviders: (slug: string, on: boolean) => call(HarnessesAPI.SetAllProviders(slug, on) as Cancellable<void>, () => {}),
      launch: (slug: string, routeKey: string, profileSlug: string) => call(HarnessesAPI.Launch(slug, routeKey, profileSlug) as Cancellable<unknown>, (r) => r as never),
    },
    usage: {
      snapshots: (force: boolean) => call(UsageAPI.Snapshots(force) as Cancellable<unknown>, (r) => r as never),
      setMode: (mode: 'auto' | 'on' | 'off') => call(UsageAPI.SetMode(mode) as Cancellable<void>, () => {}),
      setBackend: (backend: 'off' | 'native' | 'codexbar') => call(UsageAPI.SetBackend(backend) as Cancellable<void>, () => {}),
      mode: () => call(UsageAPI.Mode() as Cancellable<unknown>, (r) => r as never),
    },
    favourites: {
      list: () => call(FavouritesAPI.List() as Cancellable<unknown>, (r) => r as never),
      pin: (routeKey: string) => call(FavouritesAPI.Pin(routeKey) as Cancellable<void>, () => {}),
      unpin: (routeKey: string) => call(FavouritesAPI.Unpin(routeKey) as Cancellable<void>, () => {}),
    },
    settings: {
      get: () => call(SettingsAPI.Get() as Cancellable<unknown>, (r) => r as never),
      set: (s) => call(SettingsAPI.Set(s) as Cancellable<void>, () => {}),
      shellSnippets: () => call(SettingsAPI.ShellSnippets() as Cancellable<unknown>, (r) => r as never),
    },
    signin: {
      start: (provider: string) =>
        call(SignInAPI.Start(provider) as Cancellable<unknown>, (r) => r as {
          flow_id: string
          verification_uri: string
          user_code: string
          paste_required: boolean
        }),
      confirm: (provider: string, flowId: string, accountName: string) =>
        call(SignInAPI.Confirm(provider, flowId, accountName) as Cancellable<void>, () => {}),
      submitCode: (provider: string, flowId: string, code: string) =>
        call(SignInAPI.SubmitCode(provider, flowId, code) as Cancellable<void>, () => {}),
      cancel: (provider: string, flowId: string) =>
        call(SignInAPI.Cancel(provider, flowId) as Cancellable<void>, () => {}),
      saveAPIKey: (provider: string, accountName: string, apiKey: string) =>
        call(SignInAPI.SaveAPIKey(provider, accountName, apiKey) as Cancellable<void>, () => {}),
    },
    window: {
      openSettings: () => call(WindowService.OpenSettings() as Cancellable<void>, () => {}),
      closeSettings: () => call(WindowService.CloseSettings() as Cancellable<void>, () => {}),
      hidePopover: () => call(WindowService.HidePopover() as Cancellable<void>, () => {}),
      quit: () => call(WindowService.Quit() as Cancellable<void>, () => {}),
      copyToClipboard: (text: string) => call(WindowService.CopyToClipboard(text) as Cancellable<void>, () => {}),
      openURL: (url: string) => call(WindowService.OpenURL(url) as Cancellable<void>, () => {}),
      setPopoverHeight: (height: number) => call(WindowService.SetPopoverHeight(height) as Cancellable<void>, () => {}),
      setTrayPick: (profileName: string, modelName: string, reasoning: string, provider: string) =>
        call(WindowService.SetTrayPick(profileName, modelName, reasoning, provider) as Cancellable<void>, () => {}),
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