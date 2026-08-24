---
kind: feature-spec
version: "1.0"
feature: U04-results
project: which-model-desktop
---

# U04-results — Ranking Result Components

## 1. Purpose

The four `packages/ui` components that render ranking output and profile weight
summaries: `RankCarousel` (one candidate at a time with prev/next), `RankList`
(all candidates as selectable rows), `ModelResultCard` (compact name + meta
block with an optional copy-id affordance, used by the U06 weights-view
footer context), and `ProfileWeightSparkbar` (the core/task weight bar strip
used by the U08 profiles list). All consume the canonical `RankedModel` DTO
(D00 CONTRACTS §2) and profile weight maps; props-only, no host access
(U00 SPEC §2.2). Mockup `specs/desktop/mockup/demo.dc.html` is normative for
geometry and copy (carousel/list bands lines 184–208, sparkbars lines 287–312,
view-model lines 1250–1265 and 1476–1503).

Inherits `specs/desktop/global/*` and `specs/desktop/ui/*`. Depends on: U01
(types), U02 (`Tooltip`, `cx`).

## 2. Behaviour

### 2.1 RankCarousel

1. Renders the accent band (geometry in CONTRACTS §3) containing a prev
   chevron button, a centred three-line stack, and a next chevron button.
2. Populated (`items.length > 0`, `0 <= index < items.length`), with
   `m = items[index]`: rank line is exactly `` `rank ${index + 1} of ${items.length}` ``;
   name line is `m.model_name`; meta line is exactly
   `` `${m.provider} · ${m.reasoning} · ${m.score.toFixed(2)}` ``
   (separator ` · `, score always 2dp per D00 §6).
3. Prev is disabled iff `index === 0`; next is disabled iff
   `index === items.length - 1`. Enabled clicks call `onIndex(index - 1)` /
   `onIndex(index + 1)` exactly once; disabled buttons carry the HTML
   `disabled` attribute, render the dimmed foreground (CONTRACTS §3), and
   never fire `onIndex`. The component is controlled: it never stores index
   state itself.
4. **Empty state** (`items.length === 0`): rank line `no route`, name line
   `Enable a provider`, meta line `every provider is switched off`; both
   chevrons disabled. Exact strings; no other layout change.

### 2.2 RankList

1. Renders the accent band as a vertical stack (gap 2px) of one row-button
   per item, in `items` order (already rank-ascending from the backend).
2. Each row shows, left to right: rank number (`item.rank`), `model_name`,
   right-aligned `score.toFixed(2)`, then the provider portion of
   `route_key` — the substring before the first `/` (D00 CONTRACTS §1
   grammar) — in a fixed 52px right-aligned column.
3. The row at `index` is the selected row: accent background, bright name,
   accent-300 score (CONTRACTS §3). Clicking any row calls `onPick(i)` with
   that row's array index exactly once — including re-clicking the selected
   row (the parent decides whether that is a no-op). Controlled: no internal
   selection state.
4. Empty `items` renders the band with no rows (height collapses to
   padding); no placeholder text — hosts choosing the list layout switch to
   `RankCarousel`'s empty state at the view level (U05).

### 2.3 ModelResultCard

1. Compact block: `model_name` on top (13px heading), meta line below in the
   same format as the carousel meta (§2.1.2).
2. When `onCopyId` is provided, renders a small ghost icon-button (copy
   glyph, 22×22, `aria-label="copy model id"`) after the name; clicking calls
   `onCopyId(model.model_id)` exactly once. Omitted prop ⇒ no button
   rendered at all. The card itself is non-interactive; clipboard and toast
   are the caller's job (U06).

### 2.4 ProfileWeightSparkbar

1. Renders two bar groups — `core` then `task` — separated by a 1px × 16px
   divider, all bottom-aligned in a 24px-tall strip (CONTRACTS §3). Entries
   render in the array order given; callers pass only weighted entries
   (value ≥ 1); the component renders whatever it is given, clamping values
   to 0..5 for height purposes only.
2. Bar height is exactly `round(4 + value / 5 * 20)` px (so 1→8, 3→16,
   5→24). Core bars use `--color-accent-400`, task bars `--color-accent-700`.
3. **Hover is self-managed** (Decisions): the component owns a
   `hoveredKey: string | null` state; pointer-enter on a bar's 5×24 hit area
   sets it, pointer-leave clears it (only if still the hovered key). The
   hovered bar shows a U02 `Tooltip` chip above it with the exact content
   `` `${key}  ${value} / 5` `` (two spaces between key and value; value is
   the unclamped given value). No `hoveredKey`/`onHover` props exist.
4. Both groups empty ⇒ renders the empty 24px strip with the divider
   omitted; never throws (U00 SPEC §3).

### 2.5 Common

- All four live in `packages/ui/src/components/` per U00 CONTRACTS §2 (one
  folder each, CSS modules, exported `<Name>Props`), and are re-exported
  from the `packages/ui` barrel.
- Inline `style` is used only for prop-computed values: sparkbar heights,
  and nothing else — all fixed geometry lives in the CSS modules.
- No score arithmetic beyond `toFixed(2)` display formatting (D00 SPEC).

## 3. Error behaviour

- Components never throw on empty arrays (§2.1.4, §2.2.4, §2.4.4).
- `RankCarousel` with an out-of-range `index` on a non-empty `items` clamps
  the DISPLAYED index to `[0, items.length - 1]` for rendering and bound
  checks; it never calls `onIndex` to self-correct.
- Callbacks are optional only where marked (`onCopyId`); required callbacks
  are invoked unconditionally on interaction — no internal guards beyond the
  disabled states in §2.1.3.

## 4. Accessibility

- Carousel chevrons are `<button type="button">` with
  `aria-label="previous rank"` and `aria-label="next rank"`; the disabled
  state is the native `disabled` attribute (focusable-skip acceptable).
- The carousel text stack is a single `aria-live="polite"` region so
  rank/name changes are announced.
- `RankList` rows are `<button type="button">` elements; the selected row
  carries `aria-current="true"`.
- Sparkbar bars expose their tooltip text as `aria-label` on the bar hit
  area (`role="img"` per bar); the strip itself is `role="group"` with
  `aria-label="profile weights"`.

## 5. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Sparkbar hover ownership | Self-managed internal state + U02 `Tooltip`; no `hoveredKey`/`onHover` props | Hover is purely presentational; no consumer (U08) needs to observe or drive it — controlled props would leak view state into pages for nothing |
| Carousel/List statefulness | Fully controlled (`index` + callback) | U05/U06 keep `resultIndex` in app state shared across layout switches |
| List route column | Provider substring of `route_key`, not `provider` field | Matches mockup `r.route`; identical value today, but keeps the column tied to the serialised route identity |
| Empty-list rendering | Bare band, no message | Mockup shows the empty state only in carousel form; U05 owns the layout choice |
| Out-of-range index | Clamp for display, never self-correct via callback | Components must not drive their own controlled props |

## 6. Out of scope

- Data fetching, TanStack Query keys, `resultIndex` state — U05/U06.
- Copy-to-clipboard + toast on `onCopyId` — U06.
- Profiles list rows embedding `ProfileWeightSparkbar` — U08.
- `Tooltip` implementation — U02.
