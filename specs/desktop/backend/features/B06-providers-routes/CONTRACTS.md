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
| `internal/service/providers_test.go` | fixtures + tests per §8 |

Import boundary: `providers.go` MAY import `internal/{config,routing,usage,usage/cache,usage/toggle,catalog/identity,pick/band}` and stdlib. It MUST NOT import `internal/usage/fetch` (the compile-time guarantee behind SPEC §2.3's "never a live fetch").

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
```

Host binding names map to `EngineHost.providers.*` (D00 §5): `list`, `setEnabled`, `reorder`, `detail`, `setRouteEnabled`, `setAllRoutes`.

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
2. Usage disabled (`toggle.ResolveUsageEnabled` ⇒ off, or backend `off`) → `usage off`; Session/Weekly/Monthly = nil, Credits/Resets/Auth = "".
3. `OfflineRead(id, ttl)` snapshot has non-nil `Failure` → `no usage data`; usage fields as in rule 2. `ttl` = `providers.<id>.cache_ttl` when > 0, else the package constant `defaultLimitsTTL = 24 * time.Hour`.
4. Otherwise compose segments joined by `" · "` (middle dot, spaces), in this order, skipping unavailable ones:
   - For each window id in {`session`, `weekly`, `monthly`} (that order) present in `snapshot.Windows`:
     `"<id> <p>%"` where `p` = `band.WindowPercent(w)` rounded half-up to int (also assigned to the matching `ProviderInfo` pointer field);
     else, when the percent is uncomputable but `w.Used` and `w.Limit > 0` are set: `"<id> <int(Used)> of <int(Limit)>"` (pointer field stays nil).
   - Credits: the first window whose `Unit` is a credit/monetary unit and `Remaining` is set → `"<int(Remaining)> credits"`; the same string goes to `ProviderInfo.Credits` (else `""`).
   - Zero segments composed → `no usage data`.

`Resets` = `"<id> <ResetHint>"` for the first of session/weekly/monthly with non-empty `ResetHint`, else `""`. `Auth` = `string(snapshot.Source)` (`oauth`/`api`/`cli`/`web`/`local`/`cache`), `""` when rules 2–3 applied. The mockup's literal strings (`session 42% · weekly 18%`, `340 credits`, `not enabled`) are the format exemplars; its `device flow` auth text is demo data, not normative.

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
| `TestProvidersList_NoFetch` | usage fields come from cache files alone: no registry/descriptor is registered, no network; a stale cache file still populates fields (OfflineRead path). Compile-level guard: the file does not import `internal/usage/fetch` (checked by an import-list test over the package via `go/parser` or equivalent) |
| `TestProviderSetEnabled_Persists`, `TestProviderSetEnabled_CatalogueOnly`, `TestProviderSetEnabled_Unknown` | toggle → reload config.toml from disk → value round-trips; a catalogue-only provider is writable; unknown id → `not_found`, no write, no event |
| `TestProvidersReorder_RoundTrip` | reorder → List order matches input → priorities on disk are 1..N; second List after reload identical |
| `TestProvidersReorder_RejectsWrongSet` | golden messages §6 rows 1–3 in order (dup, unknown, missing); config untouched, zero events |
| `TestProviderDetail_LevelsAndDefault` | Levels = table's levels only, ascending ladder order; Default on exactly the top rung; `"default"` reasoning collapses to `high` before comparison |
| `TestProviderRoutes_DisabledArithmetic` | SetRouteEnabled off adds sorted+deduped entry; RoutesOn = RoutesTotal − matched entries; unmatched stale entry subtracts nothing and survives writes; SetAllRoutes(true) deletes the key; SetAllRoutes(false) writes the full sorted list. (Rank-side exclusion is B04's cross-feature test.) |
| all mutation tests | exactly one event with the §7 payload via the emit recorder |
