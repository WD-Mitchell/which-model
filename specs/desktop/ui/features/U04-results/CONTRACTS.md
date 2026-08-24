---
kind: feature-contracts
version: "1.0"
feature: U04-results
project: which-model-desktop
---

# U04-results — Contracts

## 1. Files owned

| Folder (under `packages/ui/src/components/`) | Files |
|---|---|
| `RankCarousel/` | `RankCarousel.tsx`, `.module.css`, `.test.tsx` |
| `RankList/` | `RankList.tsx`, `.module.css`, `.test.tsx` |
| `ModelResultCard/` | `ModelResultCard.tsx`, `.module.css`, `.test.tsx` |
| `ProfileWeightSparkbar/` | `ProfileWeightSparkbar.tsx`, `.module.css`, `.test.tsx` |

Plus the four re-export lines in `packages/ui/src/index.ts`. Imports:
`RankedModel` from `@which-model/core`; `Tooltip`, `cx` from U02. No other
cross-feature imports; no `EngineHost` anywhere.

## 2. Props (exact)

```ts
import type { RankedModel } from '@which-model/core'

export interface RankCarouselProps {
  items: RankedModel[]
  index: number                 // controlled; clamped for display only
  onIndex: (i: number) => void  // fired by enabled chevrons only
}

export interface RankListProps {
  items: RankedModel[]          // rank-ascending as delivered
  index: number                 // selected row (array index)
  onPick: (i: number) => void   // fired on any row click, incl. selected
}

export interface ModelResultCardProps {
  model: RankedModel
  onCopyId?: (modelId: string) => void  // absent ⇒ no copy button rendered
}

export interface WeightEntry { key: string; value: number } // value 0..5
export interface ProfileWeightSparkbarProps {
  core: WeightEntry[]   // accent-400 bars, rendered in array order
  task: WeightEntry[]   // accent-700 bars
}
// Hover is internal (SPEC §2.4.3 / Decisions): no hoveredKey/onHover props.
```

## 3. Geometry (from the mockup; D00 CONTRACTS §6 tokens apply)

| Element | Value |
|---|---|
| Carousel/List band | bg `color-mix(accent 8%)`, border-top 1px `color-mix(accent 30%)`; padding carousel `13px 16px 14px`, list `11px 16px 12px` |
| Chevron buttons | 22×22, 12×12 stroke-1.7 svg; fg enabled `text 62%`, disabled `text 22%` |
| Carousel rank line | mono 10px, `text 55%` |
| Carousel name | `--font-heading` 500, 19px, letter-spacing −.02em, margin-top 3px |
| Carousel meta | mono 10.5px, `text 62%`, margin-top 4px |
| List rows | column gap 2px; row: flex baseline, gap 10px, padding 5px 7px, radius 6px |
| List rank col | mono 9.5px, width 10px, `text 40%` |
| List name | 12.5px; selected `var(--color-text)`, else `text 72%` |
| List score | mono 10.5px, margin-left auto; selected `var(--color-accent-300)`, else `text 45%` |
| List route col | mono 9.5px, width 52px, right-aligned, `text 40%`; text = `route_key` before first `/` |
| Selected row bg | `color-mix(accent 12%)`; others transparent |
| Card name / meta | heading 500 13px / mono 10.5px `text 62%`; copy button 22×22 ghost |
| Sparkbar strip | 24px tall, bottom-aligned; bars 5px wide, gap 2px, radius 1.5px |
| Sparkbar height | `round(4 + value/5*20)` px, value clamped 0..5 |
| Sparkbar colours | core `--color-accent-400`, task `--color-accent-700` |
| Sparkbar divider | 1px × 16px, `text 16%`, 5px gap each side; omitted when either group empty |
| Sparkbar tooltip | U02 `Tooltip` chip centred above bar (bottom 30px): surface bg, `--shadow-md`, mono 10px, padding 4px 7px |

`color-mix(X n%)` = `color-mix(in srgb, var(--color-X) n%, transparent)`;
`text n%` likewise over `--color-text`.

## 4. Exact strings

| Where | String |
|---|---|
| Carousel rank line | `rank {index+1} of {items.length}` |
| Carousel/card meta | `{provider} · {reasoning} · {score.toFixed(2)}` |
| Empty carousel | rank `no route`; name `Enable a provider`; meta `every provider is switched off` |
| Chevron aria-labels | `previous rank` / `next rank` |
| Copy button aria-label | `copy model id` |
| Sparkbar tooltip / bar aria-label | `{key}  {value} / 5` (two spaces after key) |

## 5. Test fixtures (each `<Name>.test.tsx` MUST cover)

Fixture list: 3 `RankedModel`s built from mock data shapes, e.g.
`{rank: 1, model_id: 'gpt-5.6-luna', model_name: 'GPT-5.6 Luna', provider: 'codex', reasoning: 'max', score: 92.41, route_key: 'codex/gpt-5.6-luna@max'}`.

| Case | Assertion |
|---|---|
| Carousel bounds-disable | `index=0` ⇒ prev `disabled`, click fires nothing; `index=items.length-1` ⇒ same for next; mid index ⇒ prev click → `onIndex(index-1)`, next → `onIndex(index+1)`, once each |
| Carousel content | rank line `rank 2 of 3`; name; meta `codex · max · 92.41` (from `score.toFixed(2)`) |
| Carousel empty | `items: []` ⇒ the three §4 empty strings render; both buttons `disabled` |
| List selection | selected row has accent bg class + `aria-current="true"`; clicking row 2 → `onPick(2)` once; clicking selected row still fires |
| List route col | row shows `codex` for `route_key 'codex/gpt-5.6-luna@max'` |
| Score format | `score: 90` renders `90.00` in both list and card |
| Card copy | with `onCopyId`: button present, click → `onCopyId('gpt-5.6-luna')` once; without: no button in DOM |
| Sparkbar heights | `value 1 → height 8px`, `3 → 16px`, `5 → 24px` (inline style), core vs task colour classes |
| Sparkbar tooltip | hover bar `{key:'cost', value:3}` ⇒ tooltip text `cost  3 / 5` appears; unhover ⇒ gone; no tooltip initially |
| Sparkbar empty | `core: [], task: []` ⇒ renders, no bars, no divider, no throw |

Verify: `pnpm --filter @which-model/ui test` green (U00 SPEC §2.5 stack).

## 6. External symbols referenced

| Symbol | Source |
|---|---|
| `RankedModel` | D00 CONTRACTS §2 / `@which-model/core` (U01) |
| Route key grammar (provider = text before first `/`) | D00 CONTRACTS §1 |
| `Tooltip`, `cx` | U02 |
| Score format 2dp; mono stack | D00 CONTRACTS §6 |

Consumers: U05 (landing carousel/list per `GUISettings.layout`), U06 (weights
view carousel + `ModelResultCard` footer context), U08 (sparkbars in profile
rows).
