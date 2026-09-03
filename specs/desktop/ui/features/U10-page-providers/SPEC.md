---
kind: feature-spec
version: "1.0"
feature: U10-page-providers
project: which-model-desktop
---

# U10-page-providers — Providers Settings Page

## 1. Purpose

The Providers page of the settings window: a searchable, filterable provider catalogue with deterministic sorting, enablement, drag-to-set fallback order, secure per-provider account setup, and a detail view where each model's reasoning levels can be routed on or off. It is the GUI for provider order, enablement, credentials, and `[routes.disabled]`. Lives in the app layer (`apps/desktop/src/settings/pages/Providers/`), registered in U07's page registry under `Providers`; visuals derive from the mockup (`specs/desktop/mockup/demo.dc.html`, provider list ~lines 734–754, detail ~756–782).

Depends on: U02 (Toggle, Button, DragList, useToast), U07 (settings shell, `DetailHeader`, page registry, `PageComponentProps`).

## 2. Behaviour

1. **List data and controls.** `ProvidersPage` fetches `providers.list()` under query key `['providers']`. A search input filters provider ids case-insensitively as the user types. An enabled-state select has `all`, `enabled`, and `disabled`. A sort select defaults to `enabled-first` and offers `name-asc`, `name-desc`, `models-desc`, `models-asc`, `disabled-first`, and `priority`; every tie is provider id ascending. Filtering runs before sorting. The section label is `{visible} of {total} providers`, except the unfiltered `priority` view, where it is `providers · drag to set fallback order`.

2. **Rows.** Each visible row is keyed by `ProviderInfo.id` and shows the provider's 1-based `priority`, enable toggle, provider id, live limits, distinct-model count (`{models} model[s]`), and chevron. Clicking the card except its toggle opens detail. The unfiltered `priority` view renders the full provider universe inside U02 `DragList`; every other view is static and omits the drag handle so a filtered or derived order can never submit a partial reorder.

3. **Toggle.** Row toggle calls `providers.setEnabled(id, !enabled)`. No optimistic write; the `config:changed` event invalidates `['providers']` (U00 CONTRACTS §5) and the row re-renders from the refetch.

4. **Reorder.** Reordering is available only when sort is `priority`, enabled-state is `all`, and search is empty. `DragList.onReorder(ids)` therefore always fires with the FULL new ordering of every provider id (enabled and disabled alike), and the handler calls `providers.reorder(ids)` with exactly that array. A drop on the original index is a no-op.

5. **Detail data.** `ProviderDetail` receives the provider id via U07's detail-view stack (`PageComponentProps`) and fetches `providers.detail(id)` under `['provider', id]`. The U07 `DetailHeader` shows back link `Providers`, title = provider id, blurb = the detail copy in CONTRACTS §4.

6. **Detail summary + bulk.** Let `total` = Σ levels across all `ProviderDetail.models`, `on` = those with `enabled`. A summary line `{on} of {total} routes enabled` sits left; ghost buttons `Enable all` / `Disable all` sit right and call `providers.setAllRoutes(id, true|false)`. After them sits the visible delete affordance the list's right-click menu also offers: a trash icon button calling `providers.delete(id)`, then toasting `deleted {id}` and navigating back to the list. It is disabled (dimmed, tooltip `Built-in provider — cannot be deleted`) when `ProviderDetail.builtin` — builtins ship a usage adapter and stay in the universe whatever the config says, so Delete would look like a no-op.

7. **Per-model blocks.** One block per `ProviderModel`: a 168px column with model name (clickable button opening `{ kind: 'model', id: m.model_name, fromProvider: provider.id }`), model id, and a ghost button whose label is `Disable all` when ANY level of that model is enabled, else `Enable all`; beside it, a max-width 420px column of level rows — `Toggle` + label `reasoning {level.reasoning}` + a neutral `default` tag when `level.default` is true (the model's top level). Each reasoning label is a button: click opens the `provider-model` detail (the combo's benchmarks). Toggles still call `setRouteEnabled`. Levels come from the provider catalogue (every currently available effort level), not only scored routes.

8. **Level toggles.** A level toggle calls `providers.setRouteEnabled(id, model_id, reasoning, !enabled)`. The per-model button is a batch: sequential awaited `setRouteEnabled(id, model_id, l.reasoning, target)` calls, one per level of that model, where `target = !anyOn` — NOT `setAllRoutes`, which is provider-wide. The handler runs the calls in level order inside one async function; invalidation happens once via the resulting `config:changed` event(s).

9. **Loading/empty.** While either query is pending, render nothing below the header (no spinner chrome). An empty provider universe shows the route-refresh guidance. A non-empty universe whose active controls match no providers shows `No providers match these filters.` A detail with zero models renders `0 of 0 routes enabled`.

10. **Accounts and credentials.** Detail renders configured account metadata and an `Add account` modal. Account name is required. The authentication-method select always offers `API key` and offers `OAuth` only when `ProviderDetail.oauth_supported` is true. API-key input is masked and submitted once through `signin.saveAPIKey`; the key is never copied into query data or provider config. OAuth calls `signin.start`, opens its validated URL, and waits in `signin.confirm`. `start` returns an unguessable `flow_id` that every `confirm`, `submitCode`, and `cancel` call must echo; stale identifiers are rejected and cannot affect a replacement attempt. A device code is copied and displayed when present. `paste_required` alone enables the pasted-code input and `signin.submitCode`; callback and provider-client flows close automatically after browser authorization. Cancel or detail unmount calls `signin.cancel` for that exact flow. Removing an account submits the remaining non-secret account metadata through `providers.setAccounts`; removing the final account whose ref is `which-model` also deletes the managed credential, while externally owned refs such as `cursor-agent` remain untouched.

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
| Order number | Render `ProviderInfo.priority` in every view | filtered and derived sorts retain the provider's true fallback position |
| Default sort | `enabled-first`, id ascending within each group | Enabled providers are the active backends — the page's primary answer; alphabetical remains one select away. Supersedes the mockup-era A–Z default (issue #140); view state also survives detail navigation per the deviation note below |
| Spec sync | Spec text amended in the same PR as its intentional behaviour change | Spec-as-source requires the spec to describe shipped behaviour, never trail it (issue #140) |
| Sort ties | Provider id ascending | deterministic results for equal model counts or enabled state |
| Drag availability | Full-universe priority view only | `providers.reorder` rejects subsets; static views avoid misleading handles |
| OAuth availability | Backend `ProviderDetail.oauth_supported` only | Unsupported providers must never present a dead sign-in choice |
| Credential boundary | API keys pass once to `signin.saveAPIKey`; account rows receive non-secret refs only | Provider state, errors, and rendered data must not expose credential material |
| Browser completion | `SignInStart.paste_required` distinguishes pasted-code from callback/client flows | An empty device code does not imply that manual paste is required |

## 5. Out of scope

- `DragList`, `Toggle`, `Button`, toast machinery — U02.
- Shell, `DetailHeader`, PAGE_META rendering, detail-stack navigation — U07.
- The Go side of `providers.*` and what `limits_line`/route counting mean — backend features; `MockEngineHost` (U01) suffices for tests.
- Usage meters (Providers rows show only the textual limits line) — U11/U13 render meters.

## Deviations

- **Default sort (§2.1, §4):** changed from provider id A–Z (`name-asc`) to `enabled-first` in PR #146 (issue #140). The mockup has no settings-page provider list to carry a default, so A–Z was a planning assumption, not a mockup norm; the project owner defined the intended behaviour in #140 ("default sorting for provider should be enabled -> disabled"). This section amends the spec in the same change set as the behaviour, per the spec-sync decision above.
