---
kind: feature-spec
version: "1.0"
feature: B06-providers-routes
project: which-model-desktop
---

# B06-providers-routes — ProviderService

## 1. Purpose

`internal/service/providers.go` backs the Settings "Providers" page and its detail view (mockup `providerRows` / `provModels`): an ordered, draggable provider list with enable toggles, a per-provider limits line, an "N of M routes" summary, and a per-model / per-reasoning-level route toggle grid. It owns the `providers.<id>.enabled` / `providers.<id>.priority` config keys and the `[routes.disabled]` section, and it defines the availability arithmetic that B04 (pick) and B09 (favourites) consume via B00 §6.3.

Depends on: B02. Inherits: D00, B00 (order/enabled/availability invariants §6.1–6.3 are normative here).

## 2. Behaviour

1. **Provider universe.** The set of providers = (providers appearing in the routes table) ∪ (keys of `[providers.*]` in config). A provider present only in config (e.g. configured but with no routes yet) still lists, with `RoutesTotal = 0`. A provider present only in the routes table lists with config defaults: `Enabled = false` (default-deny, B00 §6.2), `Priority = 0`.

2. **List order.** `List` returns the universe sorted per B00 §6.1: ascending `providers.<id>.priority`, ties broken by id ascending. `ProviderInfo.Priority` is the 1-based position in the returned slice (display order), not the raw TOML value.

3. **Usage fields come from cache only.** `List` populates `Auth`, `Session`, `Weekly`, `Monthly`, `Credits`, `Resets`, and the usage part of `LimitsLine` exclusively from the last cached usage snapshot via `cache.Store.OfflineRead` — it NEVER triggers a live fetch, network access, or provider-registry call, regardless of staleness. No cached snapshot (or a snapshot carrying `Failure`) ⇒ `Session/Weekly/Monthly = nil`, `Credits/Resets/Auth = ""`. Stale cached snapshots are still used (staleness surfacing is B08's job).

4. **LimitsLine.** A human one-liner mirroring the mockup rows (`session 42% · weekly 18%`): exact composition rules in CONTRACTS §5. Precedence: provider disabled → `not enabled`; usage disabled (per `toggle.ResolveUsageEnabled` / backend `off`) → `usage off`; no usable snapshot → `no usage data`; otherwise the composed window/credits summary.

5. **Route counts.** `RoutesTotal` = number of routes-table entries for the provider. `RoutesOn` = `RoutesTotal` minus the count of `[routes.disabled].<id>` entries that match a current table route (list entries matching nothing subtract nothing). This is the same arithmetic as B00 §6.3's availability set, restricted to one provider.

6. **SetEnabled.** Writes `providers.<id>.enabled`. An unknown id — not in the routes table AND not in config — is `not_found`. Idempotent writes still persist and emit (no-op detection is not required).

7. **Reorder.** `Reorder(orderedIDs)` MUST receive exactly the current provider universe, each id once (validation order + messages in CONTRACTS §6). On success it rewrites `providers.<id>.priority = index + 1` (1..N) for every provider, creating `[providers.<id>]` tables as needed without touching their other keys.

8. **Detail.** `Detail(id)` lists the provider's models from the routes table, grouped by `model_id` (order: `model_id` ascending; `ModelName` = the table's `Model`). `Levels` = the reasoning levels the routes table actually contains for that (provider, model_id) — NOT the full ladder — sorted ascending by the canonical reasoning order. `Default` is true for exactly the highest present level. The canonical order minimal < low < medium < high < xhigh < max is defined by `identity.EffortOrder` in `internal/catalog/identity/identity.go`; a table reasoning of `"default"` is collapsed via `identity.CollapseReasoning` (→ `"high"`) before comparison. `RouteLevel.Enabled` = the level is not listed in `[routes.disabled].<id>`.

9. **Route toggles.** `SetRouteEnabled(id, modelID, reasoning, on)` removes (on) or adds (off) the entry `modelID + "@" + reasoning` in `[routes.disabled].<id>`. The (provider, modelID, reasoning) triple must exist in the routes table → else `not_found`. `SetAllRoutes(id, true)` removes the `[routes.disabled].<id>` key entirely; `SetAllRoutes(id, false)` sets it to every `model_id@reasoning` the table holds for that provider. After every mutation the persisted list is deduplicated and sorted ascending (byte order).

10. **Events.** Every mutation emits exactly one `config:changed`: `SetEnabled`/`Reorder` with payload `{"section":"providers"}`; `SetRouteEnabled`/`SetAllRoutes` with `{"section":"routes"}`. Write mechanics follow B00 §2.2 verbatim.

## 3. Error behaviour

- Validation failures (`Reorder`) and unknown ids (`SetEnabled`, `Detail`, `SetRouteEnabled`, `SetAllRoutes` → `not_found`) are checked BEFORE any lock-write; a rejected call performs no write and emits no event (B00 §6.5).
- Reorder validation follows the fixed order in CONTRACTS §6 so messages are golden-testable.
- `List`/`Detail` never fail because usage is off or the cache is empty — usage fields degrade per §2.3/§2.4 (B00 §2.7).
- Cache-file corruption is invisible: `OfflineRead` never errors; the fallback snapshot's `Failure` triggers the "no usable snapshot" path.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Provider universe | routes-table providers ∪ config `[providers.*]` keys | Config-only providers stay visible/configurable; table-only providers surface with default-deny |
| Usage reads | `cache.Store.OfflineRead` only; never `fetch.FetchAll` | The list must render instantly and offline; live refresh belongs to B08 (B00 §2.7) |
| Display Priority | 1-based position after sort, raw TOML untouched until Reorder | Raw priorities may be sparse/duplicated; Reorder is the only normaliser |
| Default level | highest present level per `identity.EffortOrder`; `"default"` collapsed first | Mockup's `l === m.reasoning` tag = the model's top ladder rung; one canonical ladder source |
| Levels shown | only levels present in the routes table for the pair | Mockup's `levelsFor` slices the ladder to the model's max; real data = the table, not a synthetic ladder |
| Disabled-list hygiene | dedup + sort on every write; unmatched entries preserved but inert | Deterministic TOML diffs; a stale entry revives correctly when the route returns |
| `SetAllRoutes(true)` | delete the key, not an empty array | Keeps config.toml minimal; absence = nothing disabled |

## 5. Out of scope

- Availability-set consumption at rank time (B04; the cross-feature test "disabled route excluded from Rank" lives in B04's fixtures — B06 asserts only the list arithmetic locally).
- Live usage fetching, refresher, staleness/confidence surfacing (B08).
- Favourites `InRange` recomputation (B09, which calls into the same B00 §6.3 arithmetic).
- Provider `weight`, `cache_ttl`, credential keys — read-only inputs here; the GUI does not edit them.
