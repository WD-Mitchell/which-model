---
kind: feature-spec
version: "1.0"
feature: F18-routing
project: which-model
---

# F18-routing — Routing Join

## 1. Purpose

`internal/routing` is the join layer of `which-model`: it binds usage providers' provider-native model ids to catalog `(Model, Reasoning)` identities, producing the `Route` table that `internal/pick` consumes for availability filtering and usage-aware weighting. Routes come from three sources with fixed precedence (user-declared, live provider model lists, models.dev catalogue), are matched against the catalog with fail-loud semantics, carry the usage `WindowIDs` that gate each model, and persist to a JSON table with provenance and staleness tracking. It is the only package where the usage and catalog domains meet (master plan §3.1).

Depends on: F07, F08, F11

## 2. Behaviour

1. **Route type is canonical.** `routing.Route` and `routing.Provenance` are used verbatim from `specs/global/CONTRACTS.md §3.1` (file `internal/routing/types.go`). No field is added, renamed, or re-tagged. (`docs/plan/README.md` §4.1; `docs/plan/annex-b-catalog-port.md` §7.1)

2. **Provenance ordering.** Provenance values are ordered most- to least-authoritative: `provider_live` > `models_dev` > `user_declared`. `Route.Provenance` records the source that actually established the route. (annex-b §7.1)

3. **Sources and precedence.** Route production consumes three per-provider inputs: models.dev catalogue records (`ModelsDev` — always available, collected by F08), live provider model lists (`LiveModels` — only when the caller holds a credentialed enumeration), and user-declared routes (`UserDeclared` — from `routes.toml` or `which-model routes add`). Sources are unioned, and for each `(Provider, ModelID)` the winning entry follows precedence `user_declared` > `provider_live` > `models_dev`. (master plan §4.2; annex-b §7.1 steps 1 and 5)

4. **Provider eligibility.** Only providers whose `usage.Descriptor.Kind` is `KindSubscription` or `KindAPIKeyBilling` participate in automatic route production. `KindGateway` and `KindLocalTool` providers are excluded from auto-derivation and MAY appear only via `UserDeclared` entries. (annex-b §7.1 step 1)

5. **excluded_models.** A provider-native model id listed in the provider's `excluded_models` (providers.toml) is never turned into an auto-derived route (from either `ModelsDev` or `LiveModels`). A `UserDeclared` entry is an explicit operator override and is honored regardless of `excluded_models`. (annex-b §7.1 step 2, with step 5 taking precedence over step 2)

6. **Identity join.** Each provider-native model is matched to catalog rows by cleaned name and declared effort: `identity.CleanModelName` compares names and `identity.CollapseReasoning` compares each declared level to `row.Reasoning`, so provider `"default"` matches catalog `"high"`. A model with declared effort levels produces one route per matching declared level, in declaration order; same-name score rows for efforts the provider does not declare are outside that provider-native model's capability and do not make the match ambiguous. A non-reasoning model uses `"default"` when that matches the sole catalog identity, otherwise it adopts the sole identity's reasoning label. `Route.Model` carries the cleaned provider name and `Route.Reasoning` carries the provider-side declared label or that sole inferred label. (annex-b §7.1 step 3 and §3.2; master plan §4.2)

7. **Absent match.** A provider-native model whose cleaned name has no catalog row is skipped with a warning and an `UnroutedModel` entry — surfaced, never silent, never a hard error. A declared level with no catalog row skips only that level (same warning and entry, with the level named); the model's other levels still route. (annex-b §7.1 step 4 "Absent"; master plan §4.2)

8. **Ambiguity is a hard error.** A provider-native model with no declared effort levels is ambiguous when its cleaned name matches multiple catalog identities that it cannot distinguish. Route production for that provider fails with `*AmbiguityError` naming the provider, model id, and every matched candidate identity; its auto-derived routes are not produced until an operator supplies a manual override. Explicit provider effort levels are authoritative capability data: each matching declared effort selects its corresponding identity, while same-name identities at undeclared efforts are ignored. User-declared routes remain unaffected, and an effort-less sole candidate is deterministic rather than ambiguous. There is no guessing, no first-match choice, and no best-score choice. (annex-b §7.1 step 4 "Ambiguous" and step 5; master plan §4.2)

9. **WindowIDs binding.** `routing.BindWindowIDs` derives a route's gating windows from the provider's descriptor windows (F11 `usage.WindowSpec`, `internal/usage/descriptor.go`): every account-level window (empty `ModelScope`) is included unconditionally; every model-scoped window whose `ModelScope` contains a case-insensitive exact-or-substring match of the route's `ModelID` or cleaned `Model` is included too. A route matching zero scoped windows still gets the account-level windows; a route matching several gets all of them. Result order: account-level windows first, then matched scoped windows, both in descriptor declaration order, deduplicated. (annex-b §7.3; master plan §4.1)

10. **User-declared window overrides.** A user-declared route with explicit windows keeps them verbatim. Without explicit windows, `BindWindowIDs` runs against the provider's descriptor so declared routes still receive account-level and scoped bindings. (`docs/plan/annex-d-cli-reference.md` §2.6 `--window`; annex-b §7.1 step 5)

11. **Persistence.** `routing.SaveTable` writes the route table as JSON to `<cache_dir>/routes.json` (annex-d §4.5 cache directory — the same tree as the usage snapshot cache, per annex-b §7.1 step 6 "alongside the usage cache") using an atomic write: temp file in the same directory, write, close, rename over the target. The table records `schema_version`, `ScoresHash` (lowercase hex sha256 of the exact bytes of the scores CSV the table was built against), a per-provider `RefreshedAt` RFC3339 timestamp, and the `Routes` list. (annex-b §7.1 step 6 and §7.2)

12. **Staleness.** `Table.Stale(currentScoresHash)` reports whether the scores CSV has been regenerated since the table was built. A stale table is surfaced as a warning at read time, never a hard error — `pick` still functions against it. (annex-b §7.2; annex-d §1 item 2)

13. **Degraded sources.** The caller marks a production run degraded (`Input.Degraded`) when usage is disabled at any level (master plan §6). In a degraded run, `LiveModels` are ignored for every provider — the live source is skipped without being attempted — production uses `models_dev` + `user_declared` only, emits exactly ONE warning naming the reduced source set (not one per provider, not one per route), and records `ProvenanceModelsDev`/`ProvenanceUserDeclared` on the routes. A provider whose `LiveModels` is nil in a NON-degraded run is that provider's normal shape (no credentialed enumeration exists for it) and triggers no warning. Fewer routes from a reduced source set is NOT an ambiguity error. `ProvenanceCounts` makes the reduced mix visible to `routes verify`. (annex-b §7.1a; master plan §6.3)

14. **Refresh triggers.** A route table is rebuilt whenever any of its three inputs changes: (a) the scores CSV is regenerated (its content hash changes — the table becomes stale), (b) a provider's model list changes, (c) `providers.toml` or `routes.toml` is edited. F18 provides the rebuild and staleness machinery; the trigger wiring and `which-model routes` commands are F27's scope. (annex-b §7.2)

## 3. Error behaviour

- `ProduceRoutes` returns a non-nil error iff at least one provider hit a hard error; the returned error is the first `*AmbiguityError` in deterministic order (provider input order, then models.dev record order). `BuildResult.Errors` maps EVERY provider with a hard error to its error, so callers can surface all failures rather than only the first.
- On ambiguity, `BuildResult.Routes` contains user-declared routes for all providers plus auto-derived routes only for providers without hard errors. The caller MUST NOT persist auto-derived routes for an errored provider (annex-b §7.1 step 4); persisting the rest of the table is valid.
- `LoadTable` returns an error for unreadable or corrupt tables; a missing file yields an error for which `os.IsNotExist(err) == true`, so callers can treat "no table yet" as an empty table.
- Warnings are returned as strings in `BuildResult.Warnings` (exact strings in `specs/features/F18-routing/CONTRACTS.md` §3) and never as errors.
- Degraded source sets never produce `AmbiguityError`; absence of a source is expected and warned once, ambiguity within the sources that ARE available still fails loud (annex-b §7.1a).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Ambiguity behaviour | Hard `*AmbiguityError` only when an effort-less provider model matches multiple identities it cannot distinguish; explicit provider efforts select only their matching identities; the error lists every matched identity and aborts auto-derivation for that provider | Provider capability data is authoritative, while an effort-less multi-identity match still MUST NOT pick the first or best-scoring candidate; the operator resolves that case via a manual override |
| Degraded-source marking | Triggered by the caller's `Input.Degraded` flag (usage disabled at any level); exactly one warning per run; `LiveModels` ignored; routes carry `ProvenanceModelsDev`/`ProvenanceUserDeclared`; `ProvenanceCounts` surfaces the mix | annex-b §7.1a: per-provider/per-route warnings would bury the signal; provenance must prevent a reduced table from looking authoritative; a provider with no credentialed enumeration is normal, not degraded |
| Route file location | `<cache_dir>/routes.json` (annex-d §4.5 cache directory, same tree as the usage cache) | annex-b step 6: "alongside the usage cache"; the table is regenerable derived data, so cache semantics (state is reserved for non-regenerable records like pick history) |
| Source union vs replacement | Union of `ModelsDev` + `LiveModels`; conflicts resolved by precedence | a model absent from the live list must still route (with `models_dev` provenance) |
| Reasoning matching | F07 cleaned-name equality plus `CollapseReasoning`; explicit provider levels select their matching identities, `"default"` collapses to `"high"`, and an effort-less sole non-high candidate adopts the catalog reasoning label | Preserve exact provider capabilities without treating unsupported score efforts as candidates; a sole identity is deterministic |
| Per-level absence | A declared level with no catalog row skips only that level (warning + `UnroutedModel`); the model's other levels route | partial coverage is the common case for multi-level models; whole-model skip only when nothing matches |
| Ambiguity candidate list | Every same-name catalog identity, in catalog order, when an effort-less model has multiple indistinguishable matches | The operator needs the full set to write a correct override |
| WindowIDs scope match | Case-insensitive exact-or-substring against `ModelID` or cleaned `Model` | Claude `opus` ↔ `claude-opus-4-5-20251101` binding (annex-b §7.3) |
| WindowIDs order | Account-level windows first, then matched scoped windows, descriptor declaration order, deduplicated | matches the annex-b Claude example `["5h","sevenDayOpus"]`; deterministic golden output |
| ScoresHash definition | Lowercase hex sha256 of the exact scores-CSV bytes | same mechanism pattern as the F06 raw-CSV provenance hash (annex-b §6.2a) |
| `excluded_models` vs overrides | Exclusion applies to auto sources only; `UserDeclared` entries are honored | step 5 overrides step 2; the operator's explicit entry is intent, not a typo to re-filter |
| Duplicate user-declared entries | First wins; a duplicate adds a warning | hand-authored config; strict-but-recoverable |
| Determinism | Route order: provider input order; within a provider: models.dev record order, then live-only entries, then user-declared-only entries | identical inputs must give identical output (golden tests, reproducible evidence) |

## 5. Out of scope

- `which-model routes` CLI commands (list/add/remove/refresh/verify) — F27 (`specs/features/F27-cmd-routes/`), which consumes `ProduceRoutes`, `Table`, `ProvenanceCounts`, and routes.toml parsing.
- Scores CSV generation, raw-CSV provenance hashing, CSV I/O — F06/F09; F18 only receives catalog identities and the scores hash.
- models.dev collection, `excluded_models` parsing, provider descriptor registry — F08/F11.
- Usage snapshot fetching and credential resolution — F14/F12.
- The usage toggle (`-tags nousage` stubs, three-state enablement, degraded `pick`) — F21; F18 compiles unchanged under `-tags nousage` because it consumes only `usage.Kind` (types), which F21's stub surface keeps compiled in.
- Band evaluation, gating, and strategy scoring — F19/F20.
- Refresh scheduling and trigger detection — F27.

## 6. Catalog join optimization (#170)

Route production builds a single invocation-local cleaned-name index and matches
provider entries only against their name bucket. Buckets preserve catalog order;
no cross-call cache or input mutation is allowed. Explicit effort matching,
default/high collapse, sole-identity fallback, ambiguity candidates and errors,
provider isolation, source precedence, and route/unrouted/warning order are
unchanged. The production benchmark includes index construction and 1,000 unique
provider-native IDs drawn deterministically from the committed score identities,
with declared matching efforts and allocation reporting.
