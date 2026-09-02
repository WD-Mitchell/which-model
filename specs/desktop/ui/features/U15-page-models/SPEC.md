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

2. **List data.** Query `['catalog-models']` → `host.catalog.models()`. One row per `CatalogModel`, in returned order (name ascending). The list is the full scores catalog, not clipped to enabled providers.

3. **Search.** A filter field (placeholder `filter models`) sits under the header. Matching is case-insensitive substring on `model_name` or `model_id`. Empty query shows every row. The filter is client-side over the fetched list.

4. **Rows.** Each row is a `.row` (hover tint, pointer): display name, model id (mono, muted; omit the id line when `model_id` is empty), reasoning levels as neutral tags, integer catalog scores for intelligence / cost / speed (`—` when null), provider count, chevron. Clicking the row opens `{ kind: 'model', id: model_name }`.

5. **Empty.** Pending query → nothing below the header (no spinner chrome). Zero models after filter → `EmptyState` text `no models match` when the query is non-empty, else `no models in the catalog`.

6. **Detail.** `detail.kind === 'model'` renders the summary for that catalog name (looked up in `['catalog-models']`). `DetailHeader`: back `Models`, title = `model_name`, blurb = `model_id` or `no provider id yet`. Body: kicker `catalog scores`, reasoning tags, a three-column intel/cost/speed readout using the same formatting as the list. Back (`closeDetail`) returns to the list.

7. **Loading/missing detail.** While the list query is pending, render `loading…` under the header. If the id is not in the list after load, keep the header (title = id) and show `couldn't load this model`.

## 3. Error behaviour

- `['catalog-models']` rejection → inline `couldn't load models` with ghost `Retry` calling `refetch`.
- Navigation is client state and cannot fail.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Catalog source | Scores CSV identities via `catalog.models()`, not the models.dev universe | Ranking already treats the scores file as the catalog; models.dev is huge and provider-scoped |
| Detail payload | Reuse the list DTO (no extra round-trip) | Summary fields are already on `CatalogModel`; per-provider cost is a follow-up |
| Filter | Client-side | The catalog is small enough to fetch once; keeps the host API a single list call |
| Score display | Integer when whole, else one decimal; `—` for null | Scores CSV values are 0–100; mock fixtures may be 0–5 |
| Identity | `Detail.id` is the cleaned catalog display name | Scores and routes join on `route.Model` / `ScoreRow.Model`, not provider-native ids |

## 5. Out of scope

- Enabled-providers-only clipping — issue #102.
- Per-provider price table and Providers-page click-through — issue #101.
- Benchmarks-for-one-combo (`catalog.modelDetail`) — already U10's `provider-model` detail.
