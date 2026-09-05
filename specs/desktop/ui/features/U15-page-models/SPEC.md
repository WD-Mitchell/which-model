---
kind: feature-spec
version: "1.0"
feature: U15-page-models
project: which-model-desktop
---

# U15-page-models — Models Settings Page

## 1. Purpose

The Settings **Models** page: a catalog-wide list of models (not nested under a provider) with search, and a summary detail opened by clicking a row. Lives in `apps/desktop/src/settings/pages/Models/`, registered in U07 under `Models`. There is no mockup frame for this page; geometry follows U09/U14 list conventions (22px gutter, `.row` hover, mono kickers).

Depends on: U02 (Input, EmptyState, Tag), U07 (shell, `DetailHeader`, page registry, `PageComponentProps`). Catalog data from `EngineHost.catalog.models()` (D00 / B05).

## 2. Behaviour

1. **Nav.** U07 ranking group includes `Models` beside Favourites: `['Profiles', 'Benchmark groups', 'Models', 'Favourites']`. PAGE_META title `Models`, blurb `Every model in the catalog. Open one for its identity, reasoning levels, and catalog scores.`, no page action.

2. **List data.** Query `['catalog-models']` → `host.catalog.models()`. One row per `CatalogModel`, in returned order (name ascending). The list includes scored and discovered models, including models without benchmarks; it respects the existing enabled-provider setting.

3. **Search.** A filter field (placeholder `filter models`) sits under the header. Matching is case-insensitive substring on `model_name` or `model_id`. Empty query shows every row. The filter is client-side over the fetched list. The query and the maker/provider multi-selects live in U15's module-level Zustand store (`pages/Models/listState.ts`, session-scoped, never persisted to storage or config), so they survive the detail round-trip: U07 renders the page's list OR its detail, and the list unmounts whenever a detail is on the stack. Menu-open flags stay view-local.

4. **Rows.** Each row is a `.row` (hover tint, pointer): display name, model id (mono, muted; omit the id line when `model_id` is empty), reasoning levels as neutral tags, integer catalog scores for intelligence / cost / speed (`—` when null), provider count, chevron. Clicking the row opens `{ kind: 'model', id: model_name }`.

5. **Empty.** Pending query → nothing below the header (no spinner chrome). Zero models after filter → `EmptyState` text `no models match` when the query is non-empty, else `no models in the catalog`.

6. **Detail.** `detail.kind === 'model'` renders the shared `ModelCard` for that catalog name via query `['catalog-model', name]` (`host.catalog.model(name)`). `DetailHeader`: back label (`fromProvider` or `Models`), title = `model_name`, blurb = `model_id` or `no provider id yet`. Body:
   - Kicker `catalog scores`, reasoning tags, three-column intel/cost/speed scores. When `in_catalog` is false (e.g. brand-new provider-listed models without catalog benchmark scores yet), render `No benchmark data yet` under `catalog scores`.
   - Kicker `enabled providers`: rows for each enabled provider offering the model, with provider id, native model id, reasoning level chips, and pricing (`$X in / $Y out per 1M` or `no listed price`). Empty state: `no enabled providers offer this model`.
   - Clicking a reasoning chip on a provider row opens `{ kind: 'provider-model', provider, modelName, reasoning }` to drill into benchmarks.
   - Back (`closeDetail`) returns to the parent list (Models page list, or Provider detail when opened from Providers).

7. **Loading/missing detail.** While the model query is pending, render `loading…` under the header. If the query rejects with `not_found` (or the model has no catalog identity), show `not yet in catalog` empty state without a retry button; for other errors, show `couldn't load this model` with ghost `Retry`.
## 3. Error behaviour

- `['catalog-models']` rejection → inline `couldn't load models` with ghost `Retry` calling `refetch`.
- `['catalog-model', name]` rejection → inline `not yet in catalog` on `not_found`; other rejections show `couldn't load this model` with ghost `Retry` calling `refetch`.
- Navigation is client state and cannot fail.
## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Catalog source | Scores plus discovered provider models via `catalog.models()` | New releases must be inspectable before benchmark scores exist |
| Detail payload | Dedicated `CatalogModelDetail` from `catalog.model(name)` | Provides reachable enabled providers and per-provider pricing from models.dev |
| List control state owner | U15-owned module store (`pages/Models/listState.ts`), session lifetime | The list unmounts under any detail (U07 renders list XOR detail); only module-level state survives the round-trip (issue #142). Mirrors U06's `lib/overrides.ts` ownership pattern |
| Filter | Client-side | The catalog is small enough to fetch once; keeps the host API a single list call |
| Score display | Integer when whole, else one decimal; `—` for null; `No benchmark data yet` when unscored | Scores CSV values are 0–100; brand-new provider models have no catalog scores yet (issue #141) |
| Identity | `Detail.id` is the cleaned catalog display name or model ID | Scores and routes join on `route.Model` / `ScoreRow.Model`, and provider models match on name or ID |
## 5. Out of scope

- Enabled-providers-only clipping — issue #102.
- Benchmarks-for-one-combo (`catalog.modelDetail`) — already U10's `provider-model` detail.

## Correction — new-model visibility and maker labels

The 2026-09-05 user decision supersedes the scores-only list restriction. GLM families use maker Z.AI; unknown model names use Other rather than appearing as invented makers. The frontend fallback and mock share the production core maker helper. Discovery does not invent scores or place unbenchmarked models in scored recommendations.
