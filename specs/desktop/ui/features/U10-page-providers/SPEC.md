---
kind: feature-spec
version: "1.0"
feature: U10-page-providers
project: which-model-desktop
---

# U10-page-providers — Providers Settings Page

## 1. Purpose

The Providers page of the settings window: a draggable priority list of providers (enable, reorder-to-set-fallback-order) and a per-provider detail view where each model's reasoning levels can be routed on or off. It is the GUI for the config's provider order, provider enablement, and `[routes.disabled]`. Lives in the app layer (`apps/desktop/src/settings/pages/providers/`), registered in U07's page registry under `Providers`; visuals are normative from the mockup (`specs/desktop/mockup/demo.dc.html`, provider list ~lines 734–754, detail ~756–782).

Depends on: U02 (Toggle, Button, DragList, useToast), U07 (settings shell, `DetailHeader`, page registry, `PageComponentProps`).

## 2. Behaviour

1. **List data.** `ProvidersPage` fetches `providers.list()` under query key `['providers']` and renders one row per `ProviderInfo`, in the array order returned (already priority-sorted). The U07 header shows the PAGE_META copy (CONTRACTS §4) with no page action. Above the rows sits the section label `providers · drag to set fallback order` and a column header row: 16px handle spacer / `#` 14px / `provider` 132px / `limits` flex / `models` 112px right-aligned / 10px chevron spacer.

2. **Rows.** Each row (inside U02 `DragList`, vertical, keyed by `ProviderInfo.id`): grab-dots handle (six-dot svg, `cursor: grab`), order number = array index + 1, `Toggle` reflecting `enabled`, provider id (bright when on, dim when off), limits line (`limits_line` when enabled, the literal `not enabled` when disabled), models column `{routes_on} of {routes_total} routes`, chevron. Clicking anywhere on the row except the toggle/handle opens the detail; the toggle click stops propagation. While a row is being dragged its background is `color-mix(accent 14%)`; other rows stay transparent.

3. **Toggle.** Row toggle calls `providers.setEnabled(id, !enabled)`. No optimistic write; the `config:changed` event invalidates `['providers']` (U00 CONTRACTS §5) and the row re-renders from the refetch.

4. **Reorder.** `DragList.onReorder(ids)` fires with the FULL new ordering of every provider id (enabled and disabled alike). The handler calls `providers.reorder(ids)` with exactly that array, then toasts `provider priority: {ids.join(' → ')}` — all ids, joined with ` → ` (space-arrow-space). A drop on the original index is a no-op (no host call, no toast).

5. **Detail data.** `ProviderDetail` receives the provider id via U07's detail-view stack (`PageComponentProps`) and fetches `providers.detail(id)` under `['provider', id]`. The U07 `DetailHeader` shows back link `Providers`, title = provider id, blurb = the detail copy in CONTRACTS §4.

6. **Detail summary + bulk.** Let `total` = Σ levels across all `ProviderDetail.models`, `on` = those with `enabled`. A summary line `{on} of {total} routes enabled` sits left; ghost buttons `Enable all` / `Disable all` sit right and call `providers.setAllRoutes(id, true|false)`.

7. **Per-model blocks.** One block per `ProviderModel`: a 168px column with model name, model id, and a ghost button whose label is `Disable all` when ANY level of that model is enabled, else `Enable all`; beside it, a max-width 420px column of level rows — `Toggle` + label `reasoning {level.reasoning}` + a neutral `default` tag when `level.default` is true (the model's top level).

8. **Level toggles.** A level toggle calls `providers.setRouteEnabled(id, model_id, reasoning, !enabled)`. The per-model button is a batch: sequential awaited `setRouteEnabled(id, model_id, l.reasoning, target)` calls, one per level of that model, where `target = !anyOn` — NOT `setAllRoutes`, which is provider-wide. The handler runs the calls in level order inside one async function; invalidation happens once via the resulting `config:changed` event(s).

9. **Loading/empty.** While either query is pending, render nothing below the header (no spinner chrome). An empty provider list renders the header rows only; a detail with zero models renders the summary line `0 of 0 routes enabled`.

## 3. Error behaviour

- Any rejected mutation toasts the `ErrorDTO.message` (U00 SPEC §3); state re-syncs on the next refetch.
- Query errors render an inline line `couldn't load providers` (list) / `couldn't load {id}` (detail) with a ghost `Retry` button calling `refetch`.
- A detail id that no longer exists after refetch (`not_found`) navigates back to the list.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Reorder payload | Full ordered id array, disabled providers included | `providers.reorder` is the whole order; mockup toast joins ALL ids (`ps.map(x => x.id)`) |
| Toast copy source | `provider priority: a → b → c` verbatim from mockup line 1089 | mockup normative |
| Per-model bulk | Sequential `setRouteEnabled` per level, target `!anyOn` | `setAllRoutes` is provider-wide (D00 §5); mockup's `onAll` flips exactly that model's level keys |
| Per-model label rule | `anyOn ⇒ Disable all` else `Enable all` | mockup `allLabel` line 1154 |
| No optimistic updates | Mutate → event-driven invalidation | U00 SPEC §2.7 owns invalidation; avoids double bookkeeping |
| Order number | Render index + 1, not `ProviderInfo.priority` | during drag the visual order is the truth; both agree at rest |

## 5. Out of scope

- `DragList`, `Toggle`, `Button`, toast machinery — U02.
- Shell, `DetailHeader`, PAGE_META rendering, detail-stack navigation — U07.
- The Go side of `providers.*` and what `limits_line`/route counting mean — backend features; `MockEngineHost` (U01) suffices for tests.
- Usage meters (Providers rows show only the textual limits line) — U11/U13 render meters.
