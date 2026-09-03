---
kind: feature-contracts
version: "1.0"
feature: U15-page-models
project: which-model-desktop
---

# U15-page-models — Contracts

## 1. Files owned

| File | Contents |
|---|---|
| `apps/desktop/src/settings/pages/Models/ModelsPage.tsx` | list + summary detail |
| `apps/desktop/src/settings/pages/Models/listState.ts` | U15-owned Zustand store for the list controls (SPEC §2.3): state `{ query, selectedMakers, selectedProviders }`, actions `setQuery` / `toggleMaker` / `toggleProvider` / `clearFilters`, exported `MODELS_LIST_INITIAL` defaults. Session lifetime — never persisted to storage or config; pure state, no host imports (U06 `overrides.ts` pattern) |
| `apps/desktop/src/settings/pages/Models/ModelsPage.module.css` | list/detail styles |
| `apps/desktop/src/settings/pages/Models/ModelsPage.test.tsx` | §4 fixtures |

Plus the `Models` registry line in U07 `pages.ts`. Imports: `Input`, `EmptyState`, `Tag`, `Button`, `cx` from `@which-model/ui`; `CatalogModel` from `@which-model/core`; `PageComponentProps` / `Detail` / `PAGE_META` / `DetailHeader` from U07; `zustand` in `listState.ts` only. Query hook `useCatalogModels` lives in U05 `queries.ts`.

## 2. Exports

```ts
export function ModelsPage(props: PageComponentProps): JSX.Element
```

Queries: `['catalog-models']` → `host.catalog.models()`.

## 3. Copy

| Slot | Text |
|---|---|
| PAGE_META title / blurb / action | `Models` / `Every model in the catalog. Open one for its identity, reasoning levels, and catalog scores.` / none |
| Filter placeholder | `filter models` |
| Section kicker (list) | `catalog models` |
| Column headers | `model`, `reasoning`, `intel`, `cost`, `speed`, `providers` |
| Empty (no catalog) | `no models in the catalog` |
| Empty (filter) | `no models match` |
| List error | `couldn't load models` / `Retry` |
| Detail back | `Models` |
| Detail blurb fallback | `no provider id yet` |
| Detail kicker | `catalog scores` |
| Score labels | `intel`, `cost`, `speed` |
| Missing detail | `couldn't load this model` |
| Null score cell | `—` |

## 4. Test fixtures (vitest + `createMockEngineHost`)

- Sidebar `Models` is present under ranking; clicking it shows PAGE_META blurb and one row per distinct mock model name (8).
- Filter `opus` leaves Claude Opus 5 and hides GPT-5.6 Luna.
- Clicking Claude Opus 5 opens a detail titled `Claude Opus 5` with back `Models`; back returns to the list.
- Empty filter-miss shows `no models match`.
- Search `GPT` then open GPT-5.6 Luna's detail and return: the query, narrowed rows, and picked maker/provider multi-selects (buttons show `Maker (1)` and `Provider (1)`) all survive the round-trip (SPEC §2.3).

Verify: `pnpm --filter desktop test -- ModelsPage` (plus `pnpm -r typecheck`).
