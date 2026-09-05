---
kind: feature-contracts
version: "1.0"
feature: B06-providers-routes
project: which-model-desktop
---

# B06-providers-routes — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `internal/service/providers.go` | provider list/detail/mutation methods on `*Services`, LimitsLine composer, disabled-list helpers |
| `internal/service/routes_refresh.go` | route refresh orchestration and F18 `routing.Input` construction |
| `internal/service/provider_models.go` | bounded Cursor/Antigravity CLI model discovery and parsers |
| `internal/service/provider_models_test.go` | parser, failure-isolation, command-selection, and full refresh tests |
| `internal/service/providers_test.go` | fixtures + provider list/detail/mutation tests per §8 |

Import boundaries: `providers.go` MAY import `internal/{config,routing,usage,usage/cache,usage/toggle,catalog/identity,pick/band}` and stdlib, but MUST NOT import `internal/usage/fetch` (the compile-time guarantee behind SPEC §2.3's "never a live fetch"). `provider_models.go` MAY import `internal/{catalog/identity,routing}` and stdlib; it MUST execute only the fixed binaries and arguments in SPEC §2.11, never a shell or provider-supplied command.

## 2. Exported API (methods on `*Services`; DTOs are D00 CONTRACTS §2 — not redefined here)

```go
package service

// ProvidersList returns the provider universe (SPEC §2.1) in display order
// (SPEC §2.2), usage fields from OfflineRead only (SPEC §2.3).
func (s *Services) ProvidersList(ctx context.Context) ([]ProviderInfo, error)

// ProviderSetEnabled writes providers.<id>.enabled (SPEC §2.6).
// Unknown id -> errNotFound. Emits config:changed{"section":"providers"}.
func (s *Services) ProviderSetEnabled(ctx context.Context, id string, on bool) error

// ProvidersReorder rewrites providers.<id>.priority = index+1 for the whole
// universe (SPEC §2.7). Validation order + messages: §6.
// Emits config:changed{"section":"providers"}.
func (s *Services) ProvidersReorder(ctx context.Context, orderedIDs []string) error

// ProviderDetail returns every model currently available from the provider
// (SPEC §2.8): routes-table models UNION the models.dev catalogue, with
// every catalogue effort level (or "default" when none are declared).
// Unknown id -> errNotFound.
func (s *Services) ProviderDetail(ctx context.Context, id string) (ProviderDetail, error)

// ProviderSetRouteEnabled toggles one [routes.disabled].<id> entry
// (SPEC §2.9). Triple absent from Detail listing -> errNotFound.
// Emits config:changed{"section":"routes"}.
func (s *Services) ProviderSetRouteEnabled(ctx context.Context, id, modelID, reasoning string, on bool) error

// ProviderSetAllRoutes clears (on) or fills (off) [routes.disabled].<id>
// (SPEC §2.9). Unknown id -> errNotFound.
// Emits config:changed{"section":"routes"}.
func (s *Services) ProviderSetAllRoutes(ctx context.Context, id string, on bool) error

// RefreshRoutes rebuilds routes from models.dev plus bounded provider-native
// discovery for enabled Cursor/Antigravity providers (SPEC §2.11).
func (p *ProviderService) RefreshRoutes(ctx context.Context) error
```

Host binding names map to `EngineHost.providers.*` (D00 §5): `list`, `setEnabled`, `reorder`, `detail`, `setRouteEnabled`, `setAllRoutes`, `refreshRoutes`.

## 3. Internal helpers (shape fixed so B04/B09 tests may reuse)

```go
// providerUniverse: sorted ids per SPEC §2.1–2.2.
func (s *Services) providerUniverse() []string
// disabledRouteSet: [routes.disabled].<id> entries as a set of "model@reasoning".
func (s *Services) disabledRouteSet(id string) map[string]struct{}
// topReasoning: highest level per identity.EffortOrder after
// identity.CollapseReasoning; input = levels present in the table.
func topReasoning(levels []string) string
```

Provider-native discovery contract (SPEC §2.11):

```go
func discoverLiveProviderModelsDefault(ctx context.Context, provider string) []routing.ModelEntry
func parseCursorModelList(output string) ([]routing.ModelEntry, error)
func parseAntigravityModelList(output string) ([]routing.ModelEntry, error)
```

- Cursor accepts `Available models`, skips `auto - ...` and `Tip:` lines, and parses every other non-empty line as `<model-id> - <display-name>`.
- Antigravity skips `Fetching available models...` and parses every other non-empty line as `<model-id>\t<display-name>`.
- Model ids match `^[A-Za-z0-9._-]+$`. Duplicate identical raw rows or any row with invalid format or characters rejects the complete listing.
- Executable route model IDs: Each distinct executable route preserves a provider-native advertised raw model ID (e.g. `claude-opus-4-8-low` for effort low, `claude-fable-5-max` for context-window max without effort). When several raw rows normalize to the same `(base, effort)`, the representative raw ID is the highest-scoring variant (plain > thinking > fast > thinking-fast); exact score ties keep the first row seen in command output.
- Provider variant and effort normalization:
  - Cursor:
    - Fast mode (`-fast`) and duplicate thinking variants (`-thinking`) merge into the canonical provider-native raw ID for that effort.
    - Suffix `-max` denotes maximum context window (e.g. `1M Max`), NOT a reasoning level. It does not appear as a reasoning level. For models exposing only context-window variants (e.g. `claude-fable-5-max`), the model entry emits with empty reasoning so it adopts catalog reasoning. When an effort-bearing family also lists `-max` context-window rows (e.g. `claude-opus-4-8-max`, `kimi-k3-max`), those rows do NOT create a separate entry — the family's effort routes cover the model. An UNSUFFIXED id in an effort-bearing family (e.g. `gpt-5.3-codex` beside `-low`/`-high`/`-xhigh`) is a distinct advertised executable route (the provider's default launch target) and MUST survive alongside the effort routes with empty reasoning.
    - Suffix effort levels: `-extra-high`→`xhigh`, `-xhigh`→`xhigh`, `-high`→`high`, `-medium`→`medium`, `-low`→`low`, `-minimal`→`minimal`, `-none`→`default`.
    - Display names pass through `identity.CleanModelName`, then remove provider presentation suffixes (`Fast`, `Thinking`, `1M`, `Max`, `Maximum`, the matched effort label) and Cursor's redundant leading `Cursor `.
  - Antigravity:
    - Effort comes from the terminal id suffix after optional `-fast`: `minimal`, `low`, `medium`, `high`, `xhigh`, `max`, `extra-high`→`xhigh`, `none`→`default`. A `-max` id is effort `max` here (unlike Cursor).
    - Display names pass through `identity.CleanModelName`, then remove provider presentation suffixes (`Fast`, `Thinking`, the matched effort label). Labels with no matching effort suffix in the id — including `Max`, `Maximum`, and `1M` (e.g. `Gemini 3.1 Pro 1M`) — survive intact.
- Command failure, timeout, empty output, output beyond 1 MiB, malformed data, or an unsupported provider returns nil. No raw command output is returned, logged, or embedded in an error.

## 4. Config keys owned

| Key | Type | Meaning |
|---|---|---|
| `providers.<id>.enabled` | bool (absent ⇒ false) | B00 §6.2 default-deny |
| `providers.<id>.priority` | int | raw sort key; Reorder normalises to 1..N |
| `routes.disabled.<id>` | []string of `model_id@reasoning` | routes excluded from availability; always deduped + sorted ascending on write; key deleted when empty |

Other `providers.<id>.*` keys (`weight`, `cache_ttl`, `source_preference`, `credential_path`, `trusted_fallback_origin` — `internal/config/types.go` `ProviderConfig`) are read-only here and MUST survive B06 writes byte-identically (unknown-key preservation, D00 §2.3).

## 5. LimitsLine composition (exact; also fills Session/Weekly/Monthly/Credits/Resets/Auth)

Evaluated in order; first match wins for the line:

1. `!enabled` → `not enabled` (usage fields still populated from cache if present).
2. Usage disabled (`toggle.ResolveUsageEnabled` ⇒ off, or backend `off`) → `usage off`; Session/Weekly/Monthly = nil, Credits/Resets = "".
3. `OfflineRead(id, ttl)` snapshot has non-nil `Failure` → `no usage data`; usage fields as in rule 2. `ttl` = `providers.<id>.cache_ttl` when > 0, else the package constant `defaultLimitsTTL = 24 * time.Hour`.
4. Otherwise compose segments joined by `" · "` (middle dot, spaces), in this order, skipping unavailable ones:
   - For each window id in {`session`, `weekly`, `monthly`} (that order) present in `snapshot.Windows`:
     `"<id> <p>%"` where `p` = `band.WindowPercent(w)` rounded half-up to int (also assigned to the matching `ProviderInfo` pointer field);
     else, when the percent is uncomputable but `w.Used` and `w.Limit > 0` are set: `"<id> <int(Used)> of <int(Limit)>"` (pointer field stays nil).
   - Credits: the first window whose `Unit` is a credit/monetary unit and `Remaining` is set → `"<int(Remaining)> credits"`; the same string goes to `ProviderInfo.Credits` (else `""`).
   - Zero segments composed → `no usage data`.

`Resets` = `"<id> <ResetHint>"` for the first of session/weekly/monthly with non-empty `ResetHint`, else `""`. `Auth` first uses configured accounts with nonblank refs: kinds `oauth`, `token`, `cookie` map to `oauth`, `api`, `web`; deduplicate and join with ` + ` in that order. This metadata is independent of usage availability. Without a recognized configured account, use `string(snapshot.Source)` (`oauth`/`api`/`cli`/`web`/`local`/`cache`), or `""` when usage is disabled/missing/failed. Do not change the cached source or read credentials. The mockup's literal strings (`session 42% · weekly 18%`, `340 credits`, `not enabled`) are the format exemplars; its `device flow` auth text is demo data, not normative.

## 6. Validation error strings (exact; checked in this order)

`ProvidersReorder(orderedIDs)` — all map to `validation_failed` unless noted:

| # | Check | String |
|---|---|---|
| 1 | duplicate id in input | `providers: reorder list contains duplicate id %q` |
| 2 | input id not in universe | `providers: unknown provider %q` |
| 3 | universe id missing / length mismatch | `providers: reorder list must contain every provider exactly once (got %d, want %d)` |

Other methods: unknown provider id (`SetEnabled`, `Detail`, `SetRouteEnabled`, `SetAllRoutes`) → `errNotFound`, message `providers: unknown provider %q`; unknown route triple (`SetRouteEnabled`) → `errNotFound`, message `providers: no route %s/%s@%s` (id, modelID, reasoning).

## 7. Events emitted

| Method | Event | Payload |
|---|---|---|
| ProviderSetEnabled, ProvidersReorder | `config:changed` | `{"section":"providers"}` |
| ProviderSetRouteEnabled, ProviderSetAllRoutes | `config:changed` | `{"section":"routes"}` |

Exactly one per successful mutation; zero on validation/not-found paths (B00 §6.5).

## 8. Test fixtures (`providers_test.go`; helper = B02 `newTestServices`)

Default fixture: routes table with providers `claude`,`codex` (≥2 models × ≥2 levels each, B00 §2.9); usage-cache dir under the temp cache root, seeded per-test with hand-written `<id>.json` cache files.

| Test | Asserts |
|---|---|
| `TestProvidersList_OrderAndUniverse` | union of table+config+catalogue providers; priority asc, ties id asc; display Priority 1..N; config-only provider has RoutesTotal/Models 0; table-only provider Enabled false; Models counts distinct catalogue ∪ routed model ids |
| `TestProvidersList_LimitsLine` | golden table over §5: disabled → `not enabled`; usage off → `usage off`; no/failed cache → `no usage data`; seeded snapshot → exact composed line + pointer fields + Credits/Resets/Auth |
| `TestProvidersListAuthenticationUsesConfiguredAccounts` | OAuth overrides API transport, token overrides OAuth transport, cookie maps to web, mixed accounts deduplicate in stable order, missing usage preserves configured auth, blank refs and absent accounts retain cache fallback, cached source is unchanged |
| `TestProvidersList_NoFetch` | usage fields come from cache files alone: no registry/descriptor is registered, no network; a stale cache file still populates fields (OfflineRead path). Compile-level guard: the file does not import `internal/usage/fetch` (checked by an import-list test over the package via `go/parser` or equivalent) |
| `TestProviderSetEnabled_Persists`, `TestProviderSetEnabled_CatalogueOnly`, `TestProviderSetEnabled_Unknown` | toggle → reload config.toml from disk → value round-trips; a catalogue-only provider is writable; unknown id → `not_found`, no write, no event |
| `TestProvidersReorder_RoundTrip` | reorder → List order matches input → priorities on disk are 1..N; second List after reload identical |
| `TestProvidersReorder_RejectsWrongSet` | golden messages §6 rows 1–3 in order (dup, unknown, missing); config untouched, zero events |
| `TestProviderDetail_LevelsAndDefault` | Levels = table's levels only, ascending ladder order; Default on exactly the top rung; `"default"` reasoning collapses to `high` before comparison |
| `TestProviderRoutes_DisabledArithmetic` | SetRouteEnabled off adds sorted+deduped entry; RoutesOn = RoutesTotal − matched entries; unmatched stale entry subtracts nothing and survives writes; SetAllRoutes(true) deletes the key; SetAllRoutes(false) writes the full sorted list. (Rank-side exclusion is B04's cross-feature test.) |
| `TestParseCursorModelList`, `TestParseAntigravityModelList` | real CLI row formats become exact executable model ids, cleaned names, and reasoning levels; Cursor duplicate variants (-thinking, -fast, -max context window) merge into canonical raw model IDs |
| `TestProviderModelListRejectsMalformedOutput`, `TestProviderModelOutputCapsBytes`, `TestDiscoverLiveProviderModelsFailsClosed` | strict parsing, bounded output, and every command failure degrade to a nil provider-local live source |
| `TestDiscoverLiveProviderModelsUsesProviderCommandsAndFallback` | fixed Cursor command and Antigravity primary/fallback order; parsed results use provider-live identities |
| `TestRefreshRoutesDiscoversCursorAndAntigravityWithoutOpencodeAmbiguity`, `TestRefreshRoutesLiveDiscoveryFailureIsProviderLocal` | end-to-end service refresh adds scored Cursor/Antigravity detail models, explicit models.dev effort disambiguates OpenCode Kimi K3, and one missing live source does not suppress other providers |
| all mutation tests | exactly one event with the §7 payload via the emit recorder |

## Deviation — #183: explicit models.dev refresh

An explicit RefreshRoutes operation, after successful benchmark refresh/reload, attempts the existing bounded models.dev fetch once even when a valid catalogue is cached. A successful nonempty parsed catalogue atomically replaces `modelsdev_providers.json` before route rebuilding. A fetch failure uses a valid cached catalogue and records a generic service warning; if no usable cache exists it returns the fetch error. Failure to persist a freshly fetched catalogue records a warning and permits route rebuilding from memory. Read-only List, Detail, Addable, and the cache parser never fetch, including for an old price-less schema.

This supersedes the earlier cache-else-fetch refresh policy. There is no new timer, background network operation, or setting. Pinned regressions cover replacing a valid catalogue (including removals/pricing), one network attempt, unchanged cache on fetch failure, failure without a usable cache, and zero fetches from read-only methods.


## Refresh cancellation correction — #183 review

The caller context reaches models.dev HTTP collection. A cancelled fetch returns
cancellation before cache fallback or cache writes. Route publication checks
cancellation under its write lock; cancelled live discovery must not publish a
degraded replacement table. Pin `TestModelsDevRefreshCancellationDoesNotFallBackOrWrite`.

## Refresh data correction

Provider inventories persist as `map[string][]routing.ModelEntry` in `<CacheDir>/catalog/provider_models.json`, independently of `routes.json`. The bounded Codex model-cache reader admits visible entries and known reasoning levels only. Existing DTOs remain unchanged.

Pin `TestModelsDevRefreshFailureIsReportedWithoutTouchingCache` (supersedes the fallback-success row): one explicit failed fetch returns an error and preserves the previous cache bytes. Successful refreshes replace catalog data and routes, record a new scores hash and timestamps, then emit `catalog:changed` and `config:changed {section: routes}`.

The desktop starts `Services.StartDataRefresher(ctx)` once. It refreshes data immediately, then at `gui.benchmark_check_frequency`, checking interval changes each minute without restart; weekly is seven days. Each attempt has a three-minute deadline and shares manual refresh serialization. Failures retry at the configured interval. Context cancellation stops the loop. Pin `TestDataRefresherUsesConfiguredIntervalAndStops` (startup, 15-minute boundary, changed frequency, cancellation). This connects the previously persisted-only interval to the data flow.
