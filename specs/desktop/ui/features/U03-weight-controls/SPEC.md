---
kind: feature-spec
version: "1.0"
feature: U03-weight-controls
project: which-model-desktop
---

# U03-weight-controls — WeightRow, BalanceSlider, ComplexityScale, WeightEditor

## 1. Purpose

The four `packages/ui` components that let a user shape a scoring profile: one benchmark weight row in three visual styles (`WeightRow`), the core/task balance slider (`BalanceSlider`), the five-stop complexity slider (`ComplexityScale`), and the composition that arranges weight rows into the popover weights view and the settings profile-detail page (`WeightEditor`). All are controlled, props-only components per U00 SPEC §2.2; drag behaviour comes from U02's `usePointerFraction` hook (U00 SPEC §2.4). The mockup `specs/desktop/mockup/demo.dc.html` is normative for geometry, copy, and colours.

Depends on: U02 (theme, `usePointerFraction`, `cx`).

## 2. Behaviour

1. **WeightRow renders one benchmark weight (0–5) in one of three variants.** `variant: 'step' | 'bar' | 'slider'` selects the renderer; all three share the row frame (flex, align-center, label col, control col, value col, remove col) and the same value semantics (D00 §6: integers 0–5, 0 = removed/ignored). Geometry is contract §3 verbatim from the mockup (`isStep`/`isBar`/`isSlider` markup): step = 5 flex segments, gap 3px, 6px high, radius 2, segment `i` filled `--color-accent-500` when `i <= value`; bar = 6px track radius 3 in 12% text tint with an accent-500 fill at `value/5*100%`; slider = 3px track radius 2, accent-500 fill, 12px knob at `left: calc(pct% - 6px), top: -4.5px` with the D00 §6 knob ring.

2. **WeightRow drag.** The control column attaches `usePointerFraction` as `onPointerDown` with mapper `v = Math.round(f * 5)`; `onChange(v)` fires only when `v` differs from the current `value` prop (mockup `rowFor().onDown`). The row never mutates its own state — the host re-renders with the new `value`.

3. **WeightRow read-only.** `readOnly` renders the identical visuals with `cursor: default`, no pointer handler, no slider focusability, and no remove affordance (mockup `pfRow` on builtin profiles: `cursor: ro ? 'default' : 'pointer'`, `onDown: ro ? null : …`).

4. **WeightRow label and value columns.** Label: mono, 11.5px, ellipsis, width `labelWidth` (104 in the popover, 150 in profile detail); colour is muted text, or `--color-accent-300` when `accent` (the cost row in the popover; mockup `fg: k === 'cost' ? 'var(--color-accent-300)' : MUTED`). Value column per `valueStyle`: `'compact'` = bare digit, 12px column, right-aligned, 11px mono; `'verbose'` = 56px column showing `` `${value} / 5` `` in `--color-accent-300` when `value > 0`, else the literal `ignored` in dim text (mockup `pfRow.valText`/`valFg`; zero-value rows also dim the label, `fg: v ? MUTED : DIM`).

5. **WeightRow remove affordance.** When `onRemove` is provided, the trailing column is a 14×14 `.ib` × button (9×9 stroke icon, 40% text tint) firing `onRemove()`. When absent, a 14px-wide spacer keeps the columns aligned (mockup core rows: `<span style="width:14px">`). Core metrics are not removable in the popover weights view — WeightEditor passes `onRemove` only for task rows; profile detail renders neither button nor spacer (its rows end at the 56px value column).

6. **BalanceSlider** renders the mockup balance pair: a 14px-high flex row (gap 3), `--color-accent-500` bar with `flex: core`, a 14px round knob (D00 §6 ring), and a `--color-accent-800` bar with `flex: 100 - core`; bars are 5px high, radius 3. Above it, a mono 10px uppercase caption row: `core` left, `task` right; when `showRatio`, the centre additionally shows `` `${core} / ${100 - core}` `` in `--color-accent-300` (profile-detail `pfRatio`). Drag mapper: `v = Math.max(10, Math.min(90, Math.round(f * 20) * 5))`; `onChange(v)` only when `v !== core` (mockup `onBalanceDown`). `readOnly` as clause 3.

7. **ComplexityScale** renders the five-stop slider: 5px track, radius 3, `linear-gradient(to right, var(--color-accent-800), var(--color-accent-500))`; 5 ticks (1×13px, 20% text tint, top −4px) at `i*25%` for `i` in 0..4; a 15px knob (top −5px, D00 §6 ring) at `left: calc(stop*25% - 7px)`. Below: mono 9px 45%-tint labels — `simple action` left, `planning` right (margin-top 9px) — and, when `profileName` is given, a centred heading-font 15px `--color-accent-300` name 15px below. Drag mapper: `s = Math.round(f * 4)`; `onStop(s)` only when `s !== stop` (mockup `onScaleDown`).

8. **WeightEditor** composes the popover weights section (mockup `isWeights` block) and the profile-detail variant from row descriptors — it holds no weight state and does no math beyond `value/5` rendering handled by WeightRow. It renders in order: core section header (copy contract §4) with `sectionPcts.core` right-aligned; one WeightRow per `coreRows` entry (no `onRemove`, 14px spacer); task section header with `sectionPcts.task`; one WeightRow per `taskRows` entry (with `onRemove`); and — popover variant only — the add/revert row: `+ Add metric` ghost button (`onToggleAdd`), `Revert` ghost button right-aligned (`onRevert`), and when `addOpen` a bottom-anchored popup (180px, surface bg, `--shadow-md`, radius 8, max-height 150px scroll) listing `addable` keys as mono 11.5px items; clicking one fires `onAddMetric(key)` (host semantics: add at weight 3 and close, per mockup). Empty `addable` renders an empty popup surface; empty row arrays render just the headers.

9. **WeightEditor variants.** `variant: 'popover'` = labelWidth 104, `valueStyle 'compact'`, `accent` on the `cost` row, task rows removable, add/revert row shown. `variant: 'profile-detail'` = labelWidth 150, control column `max-width: 300px`, `valueStyle 'verbose'`, no remove buttons or spacers, no add/revert row, and `readOnly` cascades to every row (builtin profiles). Row order is exactly the array order given.

10. **Keyboard.** Every interactive WeightRow control, BalanceSlider, and ComplexityScale is a focusable `[role=slider]` with `aria-valuemin/max/now` (U00 SPEC §2.6). ArrowRight/ArrowUp = +1 step, ArrowLeft/ArrowDown = −1 step, clamped to range; a step is 1 for weights (0..5), 5 for balance (10..90), 1 stop for complexity (0..4). The change callback fires only when the clamped value differs. `readOnly` controls are not focusable.

## 3. Error behaviour

- Out-of-range `value`/`core`/`stop` props are clamped for rendering (never thrown); callbacks still emit only in-range values.
- Empty `coreRows`/`taskRows`/`addable` render headers/surfaces without rows — no crashes, no placeholder copy (U00 SPEC §3).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| One row component, three variants | `WeightRow` facade over step/bar/slider renderers | Mockup switches style globally via one setting; call sites stay identical |
| Change-only-on-diff | All drag/keyboard callbacks suppress repeats | Mockup `drag()` guards `v !== current`; avoids redundant rank refetches |
| Core rows keep a 14px spacer | Spacer instead of hidden button | Column alignment across core/task without layout shift; core metrics are not removable in the popover |
| WeightEditor is dumb composition | Row descriptors + callbacks, no weight math | Popover (U06) and profile detail (U10) wire different stores to one component |
| Ratio text is a BalanceSlider flag | `showRatio` | Same control serves popover (ends-only caption) and profile detail (`60 / 40` centre) |

## 5. Files

Per U00 CONTRACTS §2 component conventions; exact list in CONTRACTS §1. Owner: U03. Consumers: U05/U06 (popover), U10 (profile detail).
