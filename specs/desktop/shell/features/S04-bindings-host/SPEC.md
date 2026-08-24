---
kind: feature-spec
version: "1.0"
feature: S04-bindings-host
project: which-model-desktop
---

# S04-bindings-host — Service Bindings and the Wails EngineHost

## 1. Purpose

S04 is the seam between the Go engine and the TS frontend: it registers every engine service facet (plus the host-side `WindowService`) in `application.Options.Services`, generates the Wails v3 bindings that the frontend imports, and implements `EngineHost` (`D00 CONTRACTS §5`) over those bindings in `apps/desktop/src/host/wailsHost.ts`. It also owns the mock/wails host switch (S00 SPEC §2.7) and the CI surface check that keeps `host.ts` and the generated bindings in lockstep (S00 CONTRACTS §3).

Depends on: S02, S03, all B* implementations, U01 (for `packages/core` — see U00 CONTRACTS §1).

## 2. Behaviour

1. **One bound struct per EngineHost group.** `EngineHost` has nine groups (`profiles`, `pick`, `catalog`, `providers`, `harnesses`, `usage`, `favourites`, `settings`, `window`). `services.go` declares one exported struct per group — `ProfilesAPI`, `PickAPI`, `CatalogAPI`, `ProvidersAPI`, `HarnessesAPI`, `UsageAPI`, `FavouritesAPI`, `SettingsAPI` — each a thin wrapper holding `*service.Services` and delegating verbatim; `WindowService` (S00 CONTRACTS §3, owned here in `windowservice.go`) covers the `window` group. Wails derives binding module names from the struct names, so the generated JS modules map 1:1 onto host groups. No business logic lives in the wrappers: every method body is a single delegation call.

2. **Registration.** `services.go` exposes `registerServices(app *application.App, svc *service.Services, win *WindowService) []application.Service` returning `application.NewService(...)` entries for all nine structs; `main.go` (S02) passes the result to `application.Options.Services`. Every method listed in CONTRACTS §2 MUST be exported and bound; no extra exported methods may exist on the bound structs (the surface check fails on extras too).

3. **Promise translation.** Go methods return `(DTO, error)` or `error`. Wails maps these to `Promise<DTO>` / `Promise<void>`: a nil error resolves with the JSON-serialised DTO (snake_case keys per D00 §2); a non-nil error rejects. Engine errors are `*service.Error` values that JSON-serialise to the `ErrorDTO` shape `{code, message}` (D00 §2/§4); wrappers return them unmodified so the rejection payload is ErrorDTO-shaped.

4. **Bindings generation.** `task desktop:bindings` (S00 CONTRACTS §2) runs `wails3 generate bindings` with output directed to `apps/desktop/src/bindings`. Generated files are COMMITTED and regenerated only via the task (S00 SPEC §2.6 risk register) — never hand-edited, never regenerated implicitly by dev/build tasks. Regeneration is required whenever a bound struct's method set or a DTO shape changes.

5. **wailsHost.** `apps/desktop/src/host/wailsHost.ts` exports `createWailsHost(): EngineHost`. Each group property calls the corresponding generated binding module; each method awaits the binding, and rejections are normalised via `toEngineError` (CONTRACTS §4): an ErrorDTO-shaped rejection passes through typed; anything else wraps as `{code: 'io_error', message: String(err)}`. `on(event, cb)` delegates to the Wails runtime `Events.On(event, handler)` with D00 §3 event names passed verbatim (no prefixing, no renaming) and returns the unsubscribe function Wails provides. The handler unwraps the Wails event envelope so `cb` receives exactly the D00 §3 payload.

6. **Host switch.** `apps/desktop/src/host/index.ts` exports `getHost(): EngineHost` (S00 CONTRACTS §4): when `import.meta.env.MODE === 'browser'` it returns `createMockEngineHost()` from `@which-model/core/mock`; otherwise `createWailsHost()`. The result is memoised (one host instance per page). "Browser mode" is defined as running Vite with `--mode browser`; `apps/desktop/package.json` gains a script `"dev:browser": "vite --mode browser"`. `task desktop:dev` runs plain `vite` (default mode), so inside the webview the wails host is selected.

7. **Surface check.** `scripts/check-host-surface.mjs` statically compares the `EngineHost` interface in `packages/core/src/host.ts` against the exported functions of the generated binding modules and exits 1 on any missing or extra method (CONTRACTS §5). Wired as root `package.json` script `"check:host"` and executed by `task desktop:build` before `wails3 build`.

8. **Gate.** S04 is the integration gate: after it lands, `task desktop:dev` shows the popover listing REAL profiles from `config.toml` and settings pages reading real data; U05–U14 stop depending on the mock at runtime.

## 3. Error behaviour

- A bound method never panics across the binding boundary; engine panics are a bug, not a contract path.
- `wailsHost` guarantees every rejection reaching UI code is an `EngineError` (ErrorDTO shape). UI code may switch on `code` using the D00 §4 closed enum.
- `check:host` mismatch fails the build loudly; it never auto-regenerates bindings.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Wrapper-per-group over binding `service.Services` directly | 9 structs mirroring EngineHost groups | Binding names match host groups exactly; keeps `wailsHost.ts` a mechanical map and makes the surface check trivial |
| Bindings committed | regenerated only via `task desktop:bindings` | Wails alpha churn isolation (S00 SPEC §2.6); reviewable diffs |
| Host switch key | `import.meta.env.MODE === 'browser'` | S00 SPEC §2.7 / CONTRACTS §4 — one bundle, no build flags |
| Error normalisation in TS, not Go | `toEngineError` in wailsHost | Go side stays plain `(DTO, error)`; single choke point for shape guarantees |
| Surface check parses, doesn't execute | regex/TS-parse of `host.ts` + bindings exports | No runtime deps; runs in CI on any platform without a webview |

## 5. Out of scope

- Contents of `packages/core/src/host.ts` / `mock.ts` (D00/U01). Tray, windows, `EmitFunc` bridge (S02/S03). Hotkey/login-item/clipboard integrations and the `host:notice` event (S05). Query wiring over the host (U05+).
