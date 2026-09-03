---
kind: feature-contracts
version: "1.0"
feature: U10-page-providers
project: which-model-desktop
---

# U10-page-providers — Contracts

## 1. Files owned

| File (under `apps/desktop/src/settings/pages/providers/`) | Contents |
|---|---|
| `ProvidersPage.tsx` | list view (SPEC §2.1–2.4) |
| `ProviderDetail.tsx` | detail view (SPEC §2.5–2.8) |
| `providers.module.css` | both views' styles |
| `ProvidersPage.test.tsx`, `ProviderDetail.test.tsx` | §5 fixtures |

Plus the registry entry for `Providers` in U07's page registry. Imports: `Toggle`, `Button`, `DragList`, `useToast`, `cx` from `@which-model/ui` (U02); DTOs from `@which-model/core`; `PageComponentProps` from U07. No other cross-feature imports.

## 2. Props (exact)

```ts
import type { PageComponentProps } from '../../registry' // U07 CONTRACTS — cited, not redefined

// Both components take PageComponentProps unchanged (host, detailId,
// openDetail(id), closeDetail). ProvidersPage renders when detailId is null;
// ProviderDetail when it is a provider id — U07 owns that switch.
export function ProvidersPage(props: PageComponentProps): JSX.Element
export function ProviderDetail(props: PageComponentProps): JSX.Element
```

Queries: `['providers']` → `host.providers.list()`; `['provider', id]` → `host.providers.detail(id)` (keys per U00 CONTRACTS §6; invalidation is U05's `invalidate.ts`).

## 3. Geometry (mockup-normative; D00 CONTRACTS §6 tokens apply)

| Element | Value |
|---|---|
| Section label | mono 9px, letter-spacing .13em, uppercase, accent, padding `0 22px 6px` |
| List controls | flex center gap 10px, padding `0 22px 12px`; search grows to 240px; native `wmsel` enabled-state and sort controls |
| Column header row | flex gap 12px, padding `0 22px 7px`; cols 16px / 14px / 132px / flex / 112px right / 10px; mono 9px uppercase `text 38%` |
| Row | flex center gap 12px, padding `11px 22px`, border-top 1px `text 8%`, cursor pointer |
| Grab handle | 16×16 `.ib`, `text 35%`, cursor grab; 11×11 svg, six 1px-radius dots at (4,8)×(2.5,6,9.5) |
| Order # | mono 11px, width 14px, `text 40%` |
| Provider cell | width 132px, gap 10px: `.sw` toggle + id mono 12.5px (`text 88%` on / `text 45%` off) |
| Limits | flex 1, mono 11.5px, `text 62%` |
| Models col | width 112px, mono 10.5px, `text 45%`, right |
| Chevron | 10×10 svg stroke 1.6, `text 38%` |
| Drag highlight | dragged row bg `color-mix(accent 14%)` |
| Detail summary row | flex gap 8px, padding `0 22px 14px`; summary mono 11px `text 45%`; buttons right (`margin-left:auto`, gap 6px), ghost 11px padding `2px 7px`; `Disable all` fg `text 55%` |
| Model block | flex start gap 16px, padding `13px 22px`, border-top 1px `text 8%` |
| Model col | width 168px: name 12.5px `text 88%`; id mono 10.5px `text 45%` margin-top 4px; button ghost 11px `text 62%` margin-top 7px |
| Levels col | flex 1, column, max-width 420px; row: gap 10px, padding `7px 0`, border-top 1px `text 8%`; label mono 11.5px (`text 88%` on / `text 45%` off); `default` tag neutral mono 8.5px padding `0 5px` |

`text n%` / `accent n%` = `color-mix(in srgb, var(--color-X) n%, transparent)`.

## 4. Exact strings

| Where | String |
|---|---|
| PAGE_META title / blurb / action | `Providers` / `Drag to set priority — highest at the top. Default-deny: a provider is never read until you enable it. Open one to choose which of its models may be routed to.` / none |
| Search | placeholder and accessible label `Search providers`; case-insensitive id substring |
| Enabled filter | `All providers` / `Enabled` / `Disabled` |
| Sort options | `Name A–Z` / `Name Z–A` / `Models high–low` / `Models low–high` / `Enabled first` / `Disabled first` / `Priority (drag)` |
| Section label | `{visible} of {total} provider[s]`; priority/all/empty-search mode uses `providers · drag to set fallback order` |
| Model count | `{models} model` when 1, otherwise `{models} models` |
| Disabled limits cell | `not enabled` |
| Reorder payload | full ordered id array, only from the priority/all/empty-search view |
| Detail blurb | `Models {id} can serve. Each reasoning level routes separately — switch off the ones the picker should not consider. Click a level to see its benchmarks.` |
| Detail back link | `Providers` |
| Summary | `{on} of {total} routes enabled` |
| Bulk buttons | `Enable all` / `Disable all` (provider-wide and per-model) |
| Detail delete | icon 13px trash (`.ib` 24px box); tooltip `Delete {id}`, disabled tooltip `Built-in provider — cannot be deleted`; success toast `deleted {id}` |
| Accounts heading/action | `Accounts` / `Authentication for this provider` / `Add account` |
| Empty accounts | `No accounts yet. Add an account to authenticate this provider.` |
| Authentication methods | `OAuth` only when `oauth_supported`; `API key` always |
| API-key field/hint | masked `API key`, placeholder `Paste API key`; `Stored in the system keychain when available, otherwise in a private local file.` |
| Browser completion | device code when present; pasted-code input only when `paste_required`; otherwise `Complete sign-in in the opened browser.` |
| Level label | `reasoning {reasoning}` |
| Default tag | `default` |
| Level click | opens `{ kind: 'provider-model', provider, modelName, reasoning }` |
| Error lines | `couldn't load providers` / `couldn't load {id}` / `Retry` |
| Empty benchmarks | `No benchmarks for this model and reasoning level yet.` |

## 5. Test fixtures (vitest + `createMockEngineHost`, U01 CONTRACTS §4)

Mock providers: claude/codex/copilot enabled, cursor disabled (mock defaults).

| Case | Assertion |
|---|---|
| Row render | enabled mock providers (claude, codex, copilot) render before the disabled ones by default; disabled rows show `not enabled`; model cell uses `{models} model[s]` |
| Search | mixed-case substring narrows the rendered provider ids immediately |
| Enabled filter | disabled/enabled/all render the exact matching subsets |
| Name sorts | default is enabled-first (enabled group id-ascending, then disabled); explicit Z–A and A–Z sort purely by provider id, independent of backend priority |
| Model-count sorts | high–low and low–high use distinct model counts with id-ascending ties |
| Enabled-state sorts | enabled-first and disabled-first group correctly with id-ascending ties |
| Priority sort | restores backend priority order and the drag section label |
| Toggle | click claude toggle → `setEnabled('claude', false)` once; row click NOT fired (no `openDetail`) |
| Reorder | simulate `DragList` reorder codex→top → `providers.reorder(['codex','claude','copilot','cursor'])` with the FULL id array once, and toast text `provider priority: codex → claude → copilot → cursor` |
| Reorder no-op | drop at original index → no host call, no toast |
| Open detail | row click → `openDetail('claude')` |
| Detail summary | computed `{on} of {total} routes enabled` matches mock levels; `Enable all` → `setAllRoutes('claude', true)`; `Disable all` → `(…, false)` |
| Per-model batch | model with mixed levels: button label `Disable all`; click → one `setRouteEnabled('claude', modelId, l, false)` per level, in level order, and NO `setAllRoutes` call; all-off model shows `Enable all` and batches `true` |
| Level toggle | click one level → `setRouteEnabled(id, modelId, reasoning, !enabled)` once; `default` tag only on the level with `default: true` |
| Level click | click `max` on Claude Opus 5 → title `Claude Opus 5  (max)` and a SWE-Bench row; back via `claude` returns to accounts |
| Untested combo | click `low` on Claude Haiku 4 (catalogue-only) → empty copy `No benchmarks for this model and reasoning level yet.` |
| Error | rejecting `setEnabled` with `{code:'io_error', message:'m'}` toasts `m` |
| Account capability | unsupported provider offers only `api_key`; Cursor offers `oauth` and `api_key` |
| Browser completion | Cursor callback/client flow renders no pasted-code input or `Continue` action |
| API-key boundary | submitted key reaches `signin.saveAPIKey` once and is absent from provider detail/query output |

Verify: `pnpm --filter desktop test` green.

## 6. External symbols referenced

| Symbol | Source |
|---|---|
| `ProviderInfo`, `ProviderDetail`, `RouteLevel`, `ErrorDTO` | D00 CONTRACTS §2 |
| `providers.*` host methods | D00 CONTRACTS §5 |
| `DragList` (`onReorder(ids)`), `Toggle`, `Button`, `useToast` | U02 |
| `PageComponentProps`, `DetailHeader`, PAGE_META rendering | U07 |
| `ProviderAccount`, `ProviderDetail.oauth_supported` | D00 CONTRACTS §2 |
| `providers.setAccounts`, `signin.*` host methods | D00 CONTRACTS §5 |
| Query keys / invalidation | U00 CONTRACTS §5–6 |
