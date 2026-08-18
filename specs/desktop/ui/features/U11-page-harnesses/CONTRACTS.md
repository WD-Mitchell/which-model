---
kind: feature-contracts
version: "1.0"
feature: U11-page-harnesses
project: which-model-desktop
---

# U11-page-harnesses — Contracts

## 1. Files owned

| File (under `apps/desktop/src/settings/pages/harnesses/`) | Contents |
|---|---|
| `HarnessesPage.tsx` | list view (SPEC §2.2–2.4) |
| `HarnessDetail.tsx` | detail view (SPEC §2.5–2.10) |
| `harnesses.module.css` | both views' styles |
| `HarnessesPage.test.tsx`, `HarnessDetail.test.tsx` | §5 fixtures |

Plus the `Harnesses` entry in U07's page registry. Imports: `Toggle`, `Button`, `Input`, `Tag`, `UsageMeter`, `useToast`, `cx` from `@which-model/ui` (U02); DTOs from `@which-model/core`; `PageComponentProps` from U07.

## 2. Props (exact)

```ts
import type { PageComponentProps } from '../../registry' // U07 CONTRACTS — cited, not redefined

export function HarnessesPage(props: PageComponentProps): JSX.Element   // detailId null
export function HarnessDetail(props: PageComponentProps): JSX.Element   // detailId = harness slug
```

Queries (keys per U00 CONTRACTS §6): `['harnesses']`, `['providers']`, `['usage', false]`. Meter join: `snapshots.find(s => s.provider === p.id)`; window lookup by `UsageWindow.id` ∈ `session|weekly|monthly`, percent = `used_percent` (nil ⇒ unknown).

## 3. Geometry (mockup-normative; D00 CONTRACTS §6 tokens apply)

| Element | Value |
|---|---|
| Section label / column headers | as U10 CONTRACTS §3 rows 1–2; cols 120px / 84px / flex / 44px |
| List row | flex center gap 12px, padding `11px 22px`, border-top 1px `text 8%`, cursor pointer |
| Name cell | 120px, baseline gap 7px; name mono 12.5px `text 88%` ellipsis; `custom` tag neutral mono 8.5px padding `0 5px` |
| Pips cell | 84px, gap 8px: pips row gap 3px, each 6×6 circle — on `--color-accent-400`, off `text 14%`; count mono 10.5px (`text 62%`, `none` ⇒ `text 45%`) |
| Command cell | flex 1, mono 10px `text 45%` ellipsis |
| Actions cell | 44px right, gap 6px: trash `.ib` 22×22 (12×12 svg stroke 1.25) + chevron 10×10 `text 38%` |
| List footnote | padding `14px 22px 20px`, 11px, `text 42%`, max-width 56ch |
| Command section | padding `0 22px 18px`; label 10px uppercase .11em accent 500; note mono 10.5px `text 45%`; box `.mono .input` min-height 30px padding `7px 10px` 11.5px `text 72%` pre-wrap |
| Providers header | gap 8px pad-bottom 9px: label as above; summary mono 10.5px `text 45%`; buttons right ghost 11px `2px 7px`, `Disable all` fg `text 55%` |
| Provider card | flex center gap 14px, padding `12px 13px`, radius 8px, column-gap 6px between cards; on: bg `text 4%` + ring inset 1px `accent 22%`; off: transparent + ring `text 7%` |
| Id column | 112px: id mono 12.5px (`text 88%` on-row / `text 45%` off); `detected` tag neutral mono 8px `0 4px`; auth mono 10px ellipsis (`text 50%` globally-on / `text 34%`) |
| Meters | flex 1 gap 12px; each: label mono 9px .07em uppercase `text 38%` + right value; bar 4px radius 2 track `text 10%`; value fg `text 62%` lit / `text 34%` dim |
| Meter fill | lit: `--color-accent-500`, ≥70% `--color-accent-300`; dim: `text 20%` |
| Credits column | 138px right: credits mono 10.5px (`text 72%` globally-on / `text 45%`); resets mono 10px `text 38%` |
| Detail footnote | padding-top 14px, 11px, line-height 1.65, `text 42%`, max-width 56ch |

`text n%` / `accent n%` = `color-mix(in srgb, var(--color-X) n%, transparent)`.

## 4. Exact strings

| Where | String |
|---|---|
| PAGE_META title / blurb / action | `Harnesses` / `Detected automatically. {model_id} and {reasoning} are filled from the pick. Open one to choose which providers it may use.` / `Add custom` |
| Section label / columns | `harnesses`; `harness`, `providers`, `launch command` |
| Custom tag / detected tag | `custom` / `detected` |
| Provider count | `{n} of {providers.length}`; zero ⇒ `none` |
| Trash title / delete toast | `Remove {name}` / `removed {name}` |
| List footnote (verbatim, note the `’`) | `harnesses and their providers are read from each harness’ own config on launch` |
| Add-custom defaults | name `Custom N`; command `my-agent --model {model_id}`; toast `{name} added` |
| Detail back / blurb | `Harnesses` / `Providers this harness may use, detected from its own configuration. Switch one off to keep it out of every launch here.` |
| Command section | `launch command` + note `substituted at launch from the pick` |
| Providers section | `providers`; summary `{n} of {providers.length} enabled`; `Enable all` / `Disable all` |
| Meter labels / unknown value | `session` `weekly` `monthly`; value `{pct}%` or `—` |
| Off-globally auth line | `off globally` |
| Detail footnote | `A provider switched off here is never used when launching in this harness.` |
| Error line | `couldn't load harnesses` / `Retry` |

## 5. Test fixtures (vitest + `createMockEngineHost`, U01 CONTRACTS §4)

Mock: 4 harnesses (claude/codex/copilot builtin, cursor custom seed per mock data), providers claude/codex/copilot on + cursor off, mockup usage numbers.

| Case | Assertion |
|---|---|
| List render | 4 rows; custom tag only on custom harness; pips per provider in priority order with on/off classes; count `3 of 4` and `none` for an all-off map |
| Remove | trash on a BUILTIN row calls `harnesses.delete(slug)` once + toast `removed {name}`; row click not fired |
| Add custom | action → `harnesses.save` once with `{name:'Custom 2', slug:'custom_2', command:'my-agent --model {model_id}', builtin:false, installed:false, providers:{claude:true, codex:true, copilot:true, cursor:false}}` (N=2 given 1 existing custom) + toast `Custom 2 added`; deleting then re-adding with a surviving `Custom 2` bumps to `Custom 3` |
| Command edit (custom) | typing in the Input calls `harnesses.save` with the new `command` exactly once after 300ms of quiet (fake timers), not per keystroke; unmount mid-debounce flushes the save |
| Command read-only (builtin) | builtin detail renders a non-editable box; no `save` ever fired |
| Provider toggle | row toggle → `setProvider(slug, id, !on)` once; `Enable all` → `setAllProviders(slug, true)`; `Disable all` → `(…, false)` |
| Meter join | claude row (on, globally on, session 42) → fill accent-500 width `42%`, text `42%`; a ≥70 window → accent-300 class |
| Off-in-harness dim | provider on globally but off in harness: width keeps `{pct}%`, fill `text 20%` class, value fg dim |
| Off-globally dim | cursor row: auth text `off globally`, meter widths `0`, values `—`, credits dim — regardless of harness toggle |
| Usage failure | `usage.snapshots` rejecting `usage_unavailable` ⇒ rows render, meters `—`, no toast |
| Detected tag | builtin + `providers[id]` true ⇒ tag; custom harness ⇒ never |

Verify: `pnpm --filter desktop test` green.

## 6. External symbols referenced

| Symbol | Source |
|---|---|
| `HarnessInfo`, `ProviderInfo`, `UsageDTO`, `UsageWindow`, `ErrorDTO` | D00 CONTRACTS §2 |
| `harnesses.*`, `providers.list`, `usage.snapshots` | D00 CONTRACTS §5 |
| `Toggle`, `Button`, `Input`, `Tag`, `UsageMeter`, `useToast` | U02 |
| `PageComponentProps`, `DetailHeader`, PAGE_META rendering | U07 |
| Query keys / invalidation map | U00 CONTRACTS §5–6 |
