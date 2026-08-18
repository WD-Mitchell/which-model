---
kind: feature-spec
version: "1.0"
feature: B08-usage
project: which-model-desktop
---

# B08-usage — UsageService

## 1. Purpose

`internal/service/usage.go` is the ONLY service file that imports `internal/usage/fetch` (B00 §2.7). It turns the engine's usage subsystem into four GUI-facing capabilities: on-demand snapshots as `UsageDTO`s (Settings ▸ Usage detection rows, harness-detail meters, B06 `ProviderInfo` limits), the `[usage]` mode/backend controls (mockup `usageOpts` auto/on/off, `backendOpts` off/native/codexbar), and a background refresher emitting `usage:updated`. Provider blank-imports live in the host (D00 §2.10), never here; this file reaches descriptors only through registry IDs (`usage.Get`).

Depends on: B02 (Services, error mapper, test helper). Consumed by: B06 (LimitsLine/meters), U11, U13.

## 2. Behaviour

1. **Gate.** `Snapshots(ctx, force)` first resolves the toggle: `toggle.ResolveUsageEnabled(false, s.cfg)`. When disabled it returns `errUsageUnavailable` wrapped with the reason constant (`config`, `backend_off`, `no_providers_enabled`, `compiled_out`) — mapped to `usage_unavailable` at the boundary. No fetch machinery is touched on this path and no event is emitted.

2. **Provider selection.** The fetch set is the enabled providers (`providers.<id>.enabled == true` — default-deny, B00 §6.2) ordered per B00 §6.1: ascending `priority`, ties by id ascending. That ordered slice is passed as `fetch.FetchAll`'s `providers` argument, and the same order is the DTO output order — FetchAll's own id-ascending sort is re-ordered back to config priority order before mapping.

3. **Options.** One `fetch.Options` per call: `Backend` = `s.cfg.Usage.Backend`; `Enabled` = map built from the config providers (only explicit `true`); `MaxAge` = 60s when `force` else 15m (TTL override — `Refresh` stays false so a fresh-enough cache still serves); `Timeout` = 10s; `MaxParallel` = 0 (fetch default); `CacheDir` = `s.usageCacheDir` (see Deviations); `ShowIdentity` = true (the GUI is the credential owner's local surface; `UsageDTO` carries no Account field, so only Plan survives mapping); `Offline` = false.

4. **Serialisation.** All `FetchAll` invocations on one `Services` are serialised by an unexported mutex. Concurrent `Snapshots` callers queue; a refresher tick that finds a fetch in flight (`TryLock` fails) is SKIPPED, not queued — fetches never overlap and never pile up.

5. **Snapshot→DTO mapping** (exact table in CONTRACTS §4). Per snapshot: `Provider`, `Plan`, `Confidence`, `Stale` copy through; `Auth` = the snapshot `Source` rendered for humans (CONTRACTS §4.3); `Failure` = `Failure.Message` (fallback `Failure.Code`) or `""`. Windows are emitted in the provider's `Descriptor.Windows` spec order (`usage.Get(provider).Windows`); snapshot windows absent from the descriptor (or the whole descriptor unknown — e.g. codexbar backend) append after in snapshot order. Per window: `Synthetic` windows are DROPPED (placeholders, not lanes — same reading as F19 §2.2a); `Unlimited` → `UsedPercent` nil, `Unlimited` true; `UsageKnown == false` → window kept (reset metadata is still shown) with `UsedPercent` nil; otherwise `UsedPercent` follows the derivation chain `UsedPercent` → `Used/Limit` → `(Limit-Remaining)/Limit`, rounded half-away-from-zero to int, clamped at 0 below, NOT clamped above 100; no chain link computable → nil.

6. **Credits line.** First window in DTO order with `Unit` ∈ {`credits`, `usd`} and `UsageKnown`: `Remaining` set → `"%d credits left"` / `"$%.2f left"`; else `Used`+`Limit` set → `"%d of %d credits"` / `"$%.2f of $%.2f"`; otherwise keep scanning. No qualifying window → `""`.

7. **Resets line.** The non-synthetic window with the SOONEST non-nil `ResetsAt` wins: `"<id> resets in <dur>"` with `<dur>` per CONTRACTS §4.4. No window has `ResetsAt` → the first window in DTO order with a non-empty `ResetHint` yields `"<id> <ResetHint>"`. Neither → `""`. Per-window `UsageWindow.ResetHint` is independent: verbatim `Window.ResetHint` if set, else derived `"resets in <dur>"` from `ResetsAt`, else `""`.

8. **Warnings.** `credential.Warning`s from FetchAll are logged (`log.Printf`, one line per warning, prefix `usage:`) and dropped — they never reach a DTO or event.

9. **Mode/Backend.** `Mode(ctx)` returns `{Mode, Backend}` with Mode mapped `UsageAuto→"auto"`, `UsageTrue→"on"`, `UsageFalse→"off"` and Backend the `[usage] backend` string (empty → `"off"`). `SetMode`/`SetBackend` validate against their closed enums (else `errValidation`), map back (`on→"true"`, `off→"false"` for enabled), persist `[usage]` per B00 §2.2 (copy → MarshalTOML → AtomicWriteFile → swap), and emit exactly one `config:changed` with payload `{"section":"usage"}`. No-op writes (value unchanged) still persist and emit — uniform with other services.

10. **Refresher.** `StartRefresher(ctx, interval)` starts one goroutine: an immediate fetch, then a `time.Ticker` at `interval` (≤0 → 5m default). After EVERY completed fetch — full success or partial (failure snapshots are data) — it emits `usage:updated` `{}`. A gate-disabled result (§2.1) emits nothing but the loop keeps ticking (config may re-enable usage). `ctx.Done()` stops the ticker and exits the goroutine. A second `StartRefresher` call on the same `Services` is a no-op. Refresher fetches use `force == false`.

## 3. Error behaviour

- Disabled gate → `usage_unavailable`; message is `"usage disabled: <reason>"` with the toggle reason constant.
- `FetchAll` returning an error (only ctx cancellation can) → the same snapshots-so-far are DISCARDED and the wrapped error maps to `usage_unavailable` (B00 §3).
- `SetMode`/`SetBackend` with an unknown value → `validation_failed`, message names field and value (CONTRACTS §6); nothing written, nothing emitted.
- Per-provider failures are never errors: they arrive as failure snapshots and surface as `UsageDTO.Failure` rows.
- The refresher never returns errors; failed cycles are logged with the `usage:` prefix and skipped.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| force semantics | `MaxAge` 60s vs 15m, `Refresh` false | A ≤60s-old cache is fresh enough even for a forced UI refresh; avoids hammering providers on repeated clicks |
| DTO order | config priority order (B00 §6.1), not fetch's id sort | Matches every other provider-ordered surface (B06 list, mockup rows) |
| Window order | Descriptor.Windows spec order, unknowns appended | Stable session/weekly/monthly ordering for the mockup's three meters regardless of fetch JSON order |
| Synthetic windows | dropped from DTO | Same semantics F19 assigns: a synthesized placeholder is not a real lane; showing it would fake a 0% meter |
| Unknown-usage windows | kept, `UsedPercent` nil | UI renders "—" but can still show reset hints; nil is the D00 contract for unknown |
| ShowIdentity | true (Plan surfaces; Account has no DTO field) | The desktop user owns the credentials; mockup shows plan-derived credits copy |
| Overlap control | mutex; refresher `TryLock`-skips | "must not overlap" without unbounded queueing when a provider is slow |
| Disabled during refresh | skip emit, keep ticking | Cheap (gate check only); re-enabling usage needs no refresher restart |
| Cache dir | `cache.CacheDir()` default, test-overridable field | CLI and GUI share one snapshot cache; see Deviations |

## 5. Deviations

- **B00 §2.8 (all paths from injected `config.Paths`).** The usage snapshot cache defaults to `usage/cache.CacheDir()` (`os.UserCacheDir()/which-model/usage-cache`) so the GUI reads/writes the SAME cache as the `which-model usage` CLI. An unexported `Services` field overrides it in tests (CONTRACTS §7).

## 6. Out of scope

Provider blank-imports and refresher wiring (host, S02/D00 §2.10); `ProviderInfo` composition from these DTOs (B06); usage meters/pages (U11/U13); per-provider `[providers.*]` mutation (B06); band/pressure math (engine F19/F20).
