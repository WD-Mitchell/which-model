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

5. **Route and model counts.** `RoutesTotal` = number of routes-table entries for the provider. `RoutesOn` = `RoutesTotal` minus the count of `[routes.disabled].<id>` entries that match a current table route (list entries matching nothing subtract nothing). `Models` = the number of distinct model ids in the routes table UNION the cached models.dev catalogue under `routing.CatalogueSlugFor(id)`. A catalogue-only provider can therefore report models before routes are refreshed. Route availability arithmetic remains B00 §6.3 restricted to one provider.

6. **SetEnabled.** Writes `providers.<id>.enabled`. Every id in the provider universe from §2.1 is writable, including catalogue-only providers such as Alibaba. An id absent from config, routes, registered usage providers, and the cached models.dev catalogue is `not_found`. Idempotent writes still persist and emit (no-op detection is not required).

7. **Reorder.** `Reorder(orderedIDs)` MUST receive exactly the current provider universe, each id once (validation order + messages in CONTRACTS §6). On success it rewrites `providers.<id>.priority = index + 1` (1..N) for every provider, creating `[providers.<id>]` tables as needed without touching their other keys.

8. **Detail.** `Detail(id)` returns every model currently available from the provider: the routes-table models UNION the provider's full models.dev catalogue. Listing is independent of scores. Each model lists every reasoning level models.dev declares for it (`EffortLevels`; a single `default` when none are declared and the routes table also has none), plus any extra levels already in the routes table, deduplicated by `identity.CollapseReasoning`. Missing scores no longer omit a combo. The catalogue is read from the same cache file `Addable` uses (`<cache>/catalog/modelsdev_providers.json`) under the provider's models.dev slug (`routing.CatalogueSlugFor`: builtins map `claude`→`anthropic`, `codex`→`openai`, `copilot`→`github-copilot`; an added provider's id IS its slug). An absent or unreadable cache degrades to routes-only — never an error, never a fetch (same posture as §2.3). Models are grouped by `model_id` (order: `model_id` ascending across the union; `ModelName` = the table's `Model`, falling back to the models.dev name when the table carries none). `Levels` are sorted ascending by the canonical reasoning order. `Default` is true for exactly the highest present level. The canonical order minimal < low < medium < high < xhigh < max is defined by `identity.EffortOrder` in `internal/catalog/identity/identity.go`; a table reasoning of `"default"` is collapsed via `identity.CollapseReasoning` (→ `"high"`) before comparison. `RouteLevel.Enabled` = the level is not listed in `[routes.disabled].<id>`. models.dev is the naming authority throughout.

9. **Route toggles.** `SetRouteEnabled(id, modelID, reasoning, on)` removes (on) or adds (off) the entry `modelID + "@" + reasoning` in `[routes.disabled].<id>`. The (provider, modelID, reasoning) triple must be one of the combos `Detail` lists (catalogue ∪ routes) → else `not_found`. `SetAllRoutes(id, true)` removes the `[routes.disabled].<id>` key entirely; `SetAllRoutes(id, false)` sets it to every listed combo for that provider. After every mutation the persisted list is deduplicated and sorted ascending (byte order).

10. **Events.** Every mutation emits exactly one `config:changed`: `SetEnabled`/`Reorder` with payload `{"section":"providers"}`; `SetRouteEnabled`/`SetAllRoutes` with `{"section":"routes"}`. Write mechanics follow B00 §2.2 verbatim.

11. **Provider-native model refresh.** `RefreshRoutes` augments models.dev with credentialed local CLI discovery for enabled `cursor` and `antigravity` providers when usage is enabled. Cursor runs `cursor-agent --list-models`; Antigravity runs `agy models`, then falls back to `antigravity models`. Each successful listing becomes `routing.ProviderInput.LiveModels`, so scored models that have no models.dev provider slug can still appear in `Detail`. For providers that enumerate variants as distinct IDs (such as Cursor with context-window variants, thinking variants, fast mode, and effort levels), discovery preserves an executable provider-native raw model ID for each distinct route (e.g. `claude-opus-4-8-low`), while merging duplicate fast-mode and thinking variants into their canonical raw ID and excluding context-window suffixes (e.g. Cursor's `-max`) from being treated as reasoning levels. Discovery is fail-closed and provider-local: a missing executable, command failure, timeout, empty/oversized output, or malformed/duplicate row yields no live models for that provider and does not prevent other provider sources from rebuilding. Commands have a 15-second ceiling, stdout is capped at 1 MiB, and output is strictly validated.

## 3. Error behaviour

- Validation failures (`Reorder`) and unknown ids (`SetEnabled`, `Detail`, `SetRouteEnabled`, `SetAllRoutes` → `not_found`) are checked BEFORE any lock-write; a rejected call performs no write and emits no event (B00 §6.5).
- Reorder validation follows the fixed order in CONTRACTS §6 so messages are golden-testable.
- `List`/`Detail` never fail because usage is off or the cache is empty — usage fields degrade per §2.3/§2.4 (B00 §2.7).
- Cache-file corruption is invisible: `OfflineRead` never errors; the fallback snapshot's `Failure` triggers the "no usable snapshot" path.
- `RefreshRoutes` treats failed provider CLI discovery as that provider's unavailable live source; it continues with models.dev and every other provider. Routing ambiguity retains F18's provider-local hard-error semantics.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Provider universe | routes-table providers ∪ config `[providers.*]` keys | Config-only providers stay visible/configurable; table-only providers surface with default-deny |
| Usage reads | `cache.Store.OfflineRead` only; never `fetch.FetchAll` | The list must render instantly and offline; live refresh belongs to B08 (B00 §2.7) |
| Display Priority | 1-based position after sort, raw TOML untouched until Reorder | Raw priorities may be sparse/duplicated; Reorder is the only normaliser |
| Default level | highest present level per `identity.EffortOrder`; `"default"` collapsed first | Mockup's `l === m.reasoning` tag = the model's top ladder rung; one canonical ladder source |
| Levels shown | catalogue `EffortLevels` ∪ extra route-table levels | Listing is what the provider currently offers, not what is scored |
| Unrouted models in Detail | union the full models.dev catalogue; empty `EffortLevels` → single `default` | The detail view answers "what does this provider offer", not "what is benchmarked"; models.dev is the naming authority; no fetch, cache-only (§2.3 posture) |
| Disabled-list hygiene | dedup + sort on every write; unmatched entries preserved but inert | Deterministic TOML diffs; a stale entry revives correctly when the route returns |
| `SetAllRoutes(true)` | delete the key, not an empty array | Keeps config.toml minimal; absence = nothing disabled |
| Provider-native discovery | Enabled Cursor: `cursor-agent --list-models`; enabled Antigravity: `agy models` then `antigravity models`; feed parsed rows through F18 `LiveModels` | Neither provider has a reliable same-named models.dev catalogue slug, while the installed authenticated CLI is authoritative for models the user can launch |
| Discovery failure | Return no live source for only the failed provider; 15-second timeout, 1 MiB stdout cap, strict all-row parsing, no output in diagnostics | A broken or hostile executable must not block unrelated routes, exhaust memory, or leak provider output |
| Cursor model variants | preserve provider-native raw IDs for each executable route while merging duplicate -fast/-thinking variants and excluding -max from reasoning | Cursor exposes variants as distinct IDs; without normalization same-model variants duplicate cards, inflate route counts, and misclassify -max as reasoning, while preserving raw IDs ensures cursor --model {model_id} launches an advertised executable ID for every effort (Issue #145) |

## 5. Out of scope

- Availability-set consumption at rank time (B04; the cross-feature test "disabled route excluded from Rank" lives in B04's fixtures — B06 asserts only the list arithmetic locally).
- Live usage fetching, refresher, staleness/confidence surfacing (B08).
- Favourites `InRange` recomputation (B09, which calls into the same B00 §6.3 arithmetic).
- Provider `weight`, `cache_ttl`, credential keys — read-only inputs here; the GUI does not edit them.

## 6. Deviations / corrections

### 1. Cursor model variant normalization and executable route preservation (Issue #145)

- **Prior behavior**: Provider model discovery treated every distinct model ID string from `cursor-agent --list-models` as an independent model entry. Context-window variants (`-max`), fast-mode variants (`-fast`), and thinking variants (`-thinking`) each created separate routes and cards. A trailing `-max` was erroneously mapped to effort `max`, producing duplicate cards (e.g. `claude-fable-5-max` and `claude-fable-5-thinking-max` both with level `max`), and duplicated effort levels for same-model variants (`claude-opus-5-medium` and `claude-opus-5-medium-fast` both with `medium`).
- **Correction**: Discovery normalizes Cursor model lines by merging redundant variants:
  - Fast-mode variants (`-fast`) and duplicate thinking variants (`-thinking`) merge into the canonical provider-native raw ID for that effort.
  - Suffix `-max` is recognized as maximum context window, NOT an effort level. It is stripped from the base model ID and excluded from being surfaced as effort `max`.
  - For models with no explicit effort levels (e.g. `claude-fable-5-max`), the model entry emits with empty reasoning so it adopts catalog reasoning (`default`).
  - Provider-native raw IDs are preserved for each executable route (`claude-opus-4-8-low`, `claude-opus-4-8-medium`, `claude-opus-4-8-high`, `claude-opus-4-8-xhigh`, `claude-fable-5-max`). Because the builtin Cursor harness command `cursor --model {model_id}` lacks a `{reasoning}` argument, preserving the advertised suffixed IDs ensures `HarnessService.Launch` always invokes a valid, executable model ID for every selected effort.
  - In `Detail`, each executable route ID is listed with its corresponding level, deduplicating fast-mode, thinking, and `-max` duplicates while maintaining 1:1 parity with executable routes and `SetRouteEnabled` keys.
