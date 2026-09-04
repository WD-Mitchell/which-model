---
kind: feature-spec
version: "1.0"
feature: F14-usage-fetch
project: which-model
---

# F14 — Usage Fetch: orchestration, caching, partial failure, timeouts

## Purpose

F14 is the usage subsystem's orchestrator: one call (`FetchAll`) turns a list of requested provider IDs into a sorted, partially-failing-but-never-crashing set of `usage.Snapshot`s — honoring the enable gate, the on-disk cache (F13), credential resolution (F12), per-provider deadlines, bounded fan-out, identity redaction, and deterministic ordering. Every provider adapter (F15–F17) plugs in through the F11 registry; `FetchAll` never special-cases a provider.

## Behaviour

1. **Entry point.** `FetchAll(ctx, providers []string, opts Options) ([]usage.Snapshot, []credential.Warning, error)` (annex-a §6 `Fetch`; surface agreed with F21). `Options` (CONTRACTS §1): `Refresh`, `Offline`, `MaxAge`, `ShowIdentity`, `Enabled map[string]bool`, `Timeout`, `MaxParallel`, `CacheDir`, `StateDir`, `DisableManagedKeychain`. Only requested AND enabled providers are processed; results contain exactly those.
2. **Enable gate (L1a, default-deny).** For each requested ID: `opts.Enabled == nil` or `!opts.Enabled[id]` → the provider is skipped ENTIRELY — no cache read, no credential resolution, no fetch (annex-a §1a gate "explicitly enabled"; decision D1). F21/F24 pre-filter at the command layer with `usage.IDs()`; F14 still enforces the gate as the last line of defense.
3. **Unknown providers.** `usage.Get(id)` error → `Snapshot{Provider: id, Failure: {provider_status, "unknown provider <id>"}}` (decision D2; F24 pre-validates, but library callers may not). No fetch, no cache access.
4. **Per-provider pipeline (online).** For an enabled, known provider: (a) **cache read** unless `Refresh` or `CacheTTL == 0` — TTL = `cache.EffectiveTTL(desc.CacheTTL, opts.MaxAge)` (annex-d `--max-age`); fresh → serve (step 6); miss/stale/corrupt → continue; (b) **credentials**: `credential.ResolveProvider(ctx, id, desc.Auth, client, ManagedStore{StateDir: opts.StateDir, UseKeychain: !opts.DisableManagedKeychain})`, preserving declared source precedence before managed device-flow storage; `ErrNotFound` → `Snapshot{Failure: {login_required, "no credential found for provider <id>"}}` (no fetch, no write); hard resolver error → its Failure (message scrubbed, step 9); (c) **fetch**: `desc.Fetch(ctx, cred, client)` under a per-provider context deadline (step 5); error → failure snapshot (step 9), no cache write; success → (d) **cache write** (failure → warning, never error — annex-a §6: cache is an optimization); (e) stamp source/confidence (step 11).
5. **Timeouts.** Effective per-provider timeout = `opts.Timeout` if > 0, else `desc.Timeout` if > 0, else `DefaultTimeoutSec` (10s — annex-d `--timeout` default). Enforced with `context.WithTimeout` per provider; `context.DeadlineExceeded` → `Failure{timeout}`. A slow provider NEVER delays siblings beyond its own deadline (decision D4).
6. **Serving.** Fresh cached snapshot → stamped `Source = SourceCache`, `Confidence = "cached"`, `Stale` left false, identity as stored. Stale cached + offline → served with `Stale: true` (annex-a §6). Failures are never served from cache (cache refuses them, F13 D5).
7. **Offline mode.** `Offline: true` → no credentials, no fetch, no writes: `store.OfflineRead(id, ttl)` per provider; `fallback_unavailable` Failure passes through as the snapshot (annex-d: offline + missing cache → exit 1); cached data served with `Source = SourceCache`, `Confidence = "cached"`. `Refresh` is ignored in offline mode (decision D5).
8. **Concurrency.** Provider pipelines run via `errgroup.WithContext` + `SetLimit`: limit = `opts.MaxParallel` if > 0, else `min(len(active providers), DefaultMaxParallel)` where `DefaultMaxParallel = 8` (assignment's cap; overrides annex-a §6's 16 — decision D6). Per-provider functions NEVER return errors to the group (failures are data, step 9); the group's `Wait` error can only come from shared-`ctx` cancellation — returned as `FetchAll`'s error with whatever partial results were produced. Results are collected in a buffered channel and **sorted by Provider ID ascending** before return (decision D7), so output is deterministic regardless of completion order.
9. **Failure mapping and scrubbing.** `MapError(err) usage.Failure` (exported; also used by F24 rendering and F15+ providers for consistency): (1) `usage.AsFailure` → its `Failure`; (2) `httpkit.AsError` → `Failure{Code: e.Code, Message: <sanitised e.Error()>}` (F04 pinned surface); (3) `errors.Is(err, credential.ErrNotFound)` → `login_required`; (4) `errors.Is(err, context.DeadlineExceeded)` → `timeout`; (5) else `provider_status`. Before a Failure reaches a returned snapshot, F14 scrubs the resolved credential's `Token` and every `Extra` value out of the message (`<redacted>` substitution; decision D8) — the last line of defense for invariant 5 (canary-tested). Warnings (resolver permission warnings, cache-write failures) are aggregated per provider and appended in provider-sorted order; a warning never fails the batch.
10. **Identity redaction.** `ShowIdentity: false` (default) → `Account` and `Plan` are cleared on every RETURNED snapshot (live, cached, offline alike). The cache write happens BEFORE redaction so the cache retains full identity for later `--show-identity` runs (annex-d `--show-identity`; decision D9).
11. **Source mapping.** `SourceFor(cred usage.Credential, kind usage.Kind) usage.Source` (exported; F24 `--source` filters may reuse it): `AuthEnvVar`/`AuthFile`/`AuthKeychainGeneric`/`AuthKeychainInternet` → `SourceAPI`; `AuthOAuthDeviceFlow`/`AuthOAuthRefreshGrant` → `SourceOAuth`; `AuthCLIShellOut`/`AuthSubprocessRPC` → `SourceCLI`; `AuthBrowserCookie` → `SourceWeb`; `kind == KindLocalTool` → `SourceLocal`; anything else (incl. the zero Credential from an empty Auth chain) → `SourceAPI` (decision D10). Providers return snapshots with `Source` unset; F14 alone stamps it (F11 CONTRACTS §5).
12. **Transport.** One shared client per `FetchAll` call, built as a plain `&http.Client{}` with NO `Timeout` — per-provider deadlines come from step 5's contexts and a client-level timeout would truncate slow-but-valid providers (e.g. codex 15s; decision D4). F11's `FetchFunc` signature fixes `*http.Client` (providers port core.mjs's `requestJson` directly — see `specs/DEFERRED.md` D2), so F14 does NOT construct an `httpkit.Client`; `internal/httpkit` serves the catalog collectors (F08), not the usage adapters. Auth headers are the provider's job.
13. **Build tags.** Every file in `internal/usage/fetch/` carries `//go:build !nousage` (annex-a §1a.2). F21 mirrors `FetchAll`/`Options` in the `nousage` stub.

## Error behaviour

- `FetchAll` returns `err == nil` whenever every enabled provider produced a snapshot (even `Failure`-carrying ones); partial failure is data, not an error (decision D2).
- `err != nil` only on shared-context cancellation (caller's `ctx` cancelled/deadline) — returns partial results collected so far plus the context error.
- Per-provider failures use canonical codes: `provider_status`, `login_required`, `timeout`, `network`, `response_json`, `unsafe_credential`, `expired_credential`, `access_denied`, `device_expired`, … — whatever `MapError` derives; `fallback_unavailable` only from offline mode.
- Unknown provider → `provider_status` snapshot (never a hard error).
- Cache write failures and resolver warnings → warnings, never errors.

## Decisions

| # | Decision | Value | Rationale |
|---|---|---|---|
| D1 | `Enabled` default-deny | `nil` = no providers enabled | Annex-a §1a: "explicitly enabled" gate; `FetchAll` is safe to call with zero setup. |
| D2 | Unknown provider / partial failure | failure SNAPSHOT, err == nil | Usage output is a table of per-provider rows; a broken provider must not kill the batch (annex-a §6 "partial results"). |
| D3 | Stamping | F14 alone sets Source/Confidence/Stale | Global CONTRACTS §1.5: `Confidence=cached` and `Stale` are presentation of provenance; providers stay source-agnostic (F11 CONTRACTS §5). |
| D4 | Client timeouts | plain `&http.Client{}` (no `Timeout`) + per-provider ctx deadlines | A client-level timeout truncates valid 15s providers (codex annex-a §6); per-provider contexts give each its own budget and siblings stay independent. F11 `FetchFunc(*http.Client)` rules out an httpkit client here (`specs/DEFERRED.md` D1). |
| D5 | Offline + Refresh | offline wins, Refresh ignored | Read-only mode is an invariant (F13 D8); mixing flags silently degrades to offline. |
| D6 | MaxParallel default | 8 | Assignment's cap; overrides annex-a §6's conservative 16. |
| D7 | Result order | sorted by Provider ID | Deterministic output for tests and CLI rendering (same rule as F11 `IDs()`). |
| D8 | Message scrubbing | token + Extra values → `<redacted>` in Failure.Message | Enforces invariant 5 at the last layer that still knows the credential; canary-tested. |
| D9 | Redaction timing | write cache first, then clear Account/Plan on returned snapshots | `--show-identity` must work on subsequent cached runs (annex-d §1). |
| D10 | Source fallback | unknown/zero → `SourceAPI` | Presence-checked or auth-less providers behave like API-key providers; `SourceCache` is never produced by `SourceFor` (it's cache provenance). |
| D11 | `Options.CacheDir` | string, "" = system cache dir (`cache.New()`) | Lets tests (and embedders) redirect the cache away from the real user dir; F13's `Store{Dir}` is already the seam. |
| D12 | Managed credential options | `StateDir` defaults to platform state; keychain enabled unless explicitly disabled | Direct callers inherit the secure default while CLI/pick/desktop callers propagate `[auth].use_keychain`. |

## Out of scope

- Provider adapters (fetch bodies, endpoint allow-lists, auth headers, rate-limit backoff) — F15–F17.
- `--offline`/`--refresh-usage`/`--max-age`/`--timeout`/`--max-parallel`/`--show-identity` cobra wiring — F24; usage-toggle enforcement — F21.
- Rendering/exit-code logic (`login_required` prompting, offline exit 1) — F24.
- `nousage` stub for this package — F21.
- Any per-provider retry/backoff beyond httpkit's default 1×500ms — provider-owned (F15+).

## Review correction (#180)

The CodexBar backend follows the same online cache-first pipeline: use `cache.EffectiveTTL(15*time.Minute, opts.MaxAge)` for reads, bypass online reads only with `Refresh`, and return fresh cache entries before credential/keychain/subprocess work. A cache hit never writes, sets cache/cached provenance and `Stale=false`, and applies identity redaction only to the returned copy. Missing, corrupt, stale, or failed cache entries proceed to live fetch; failures are never written. B08's normal 15-minute and force 60-second ages therefore work without unconditional refresh.

## Review correction (#181)

Each live CodexBar worker has a context timeout of positive `Options.Timeout` or 10 seconds, bounded by its parent. The worker context covers managed credential setup and either fetch function. Worker deadline expiry produces a fixed sanitized `timeout` failure, even when the adapter returns an untyped error, without cancelling siblings. Only parent cancellation/deadline is a batch error. CodexBar's standalone adapter keeps its existing 30-second ceiling.


## Managed lookup deadline correction — #181 review

The provider budget bounds managed keychain lookup as well as the CodexBar
process. Because the OS keychain interface cannot cancel an active prompt,
return on context cancellation and discard the late result; permit at most four
outstanding such calls process-wide. Do not launch CodexBar after the lookup
exhausts the budget. Pin blocked-keychain deadline and earlier-parent tests.
