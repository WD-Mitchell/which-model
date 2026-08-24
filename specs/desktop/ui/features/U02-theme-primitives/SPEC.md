---
kind: feature-spec
version: "1.0"
feature: U02-theme-primitives
project: which-model-desktop
---

# U02-theme-primitives — Theme + Primitive Components

## 1. Purpose

The foundation layer of `packages/ui`: the vendored nocturne theme, the ported app-level CSS, the `cx` class-merge util, the `usePointerFraction` drag hook, and the 18 stateless primitives every higher UI feature (U03–U14) composes. All components follow U00 CONTRACTS §2 conventions (one folder per component, exported `<Name>Props`, controlled, props-only). Geometry cites D00 CONTRACTS §6 tokens; only geometry NOT listed there is stated here. The mockup `specs/desktop/mockup/demo.dc.html` is normative.

Depends on: U01 (types available for tests; no primitive imports `@which-model/core` at runtime).

## 2. Behaviour

1. **Theme — nocturne.css.** `src/theme/nocturne.css` is byte-identical to `specs/desktop/mockup/nocturne.css` EXCEPT line 2 (the Google-Fonts `@import url('https://fonts.googleapis.com/…Inter…')`) is deleted (U00 SPEC §2.3: offline-capable; Inter arrives via `@fontsource/inter` in app entries). No other byte changes — selectors `.btn*`, `.seg*`, `.tag*`, `.input`, tokens `--color-*`/`--font-*`/`--radius-*`/`--shadow-*` are consumed as-is.

2. **Theme — app.css.** `src/theme/app.css` ports the mockup helmet `<style>` rules verbatim (rule list closed in CONTRACTS §3): `.mono`, `.ib` (+`.off`), `.row:hover`, `.sw` (+`i`, `.on`, `.on i`, `.off`), `.scroll`, `@keyframes toastIn`, `input.wminput` (+placeholder), `select.wmsel` (+option). The two `html,body` and `a` rules from the helmet are NOT ported (app-level, owned by app entries).

3. **cx.** `cx(...args)` joins truthy string arguments with a single space; `false | null | undefined` are skipped. No object/array forms (keep it the mockup-simple util U00 CONTRACTS §2 names).

4. **usePointerFraction.** Replicates the mockup `drag()` exactly (U00 SPEC §2.4): the returned handler is attached as `onPointerDown`; on pointerdown it captures `currentTarget.getBoundingClientRect()`, registers `window` `pointermove`/`pointerup` listeners, calls `onFraction(clamp((clientX - rect.left)/rect.width, 0, 1))` immediately with the down event and on every move, removes both listeners on pointerup, and calls `preventDefault()` + `stopPropagation()` on the down event. Unmount during a drag removes the listeners (cleanup effect).

5. **Button.** Renders `<button>` with nocturne classes `btn btn-primary|btn-secondary|btn-ghost` per `variant`. `size='sm'` adds a module class overriding to the mockup's compact metrics (`font-size:12px;padding:5px 11px`; ghost callers pass their own smaller paddings via `className`). Disabled uses native `disabled` (nocturne `.btn:disabled`); no click fires.

6. **SplitButton.** The launch control (mockup lines 213–216): a bordered pill (`1px solid var(--color-accent)`, `border-radius: var(--radius-md)`, overflow hidden) containing a mono label segment (`Launch in {harness}` style; `padding:5px 10px;font-size:12px;color:var(--color-accent)`) firing `onMain`, and a chevron segment (`padding:5px 7px`, `border-left:1px solid color-mix(in srgb,var(--color-accent) 45%,transparent)`) toggling the menu via `onOpenChange(!open)`. `open` is controlled; when true the container background is accent-tinted (`color-mix(in srgb,var(--color-accent) 9%,transparent)`) and a `Menu`-style surface renders above/right (width 158px, padding 5px, radius 8px, `background:var(--color-surface)`, `shadow-md`) listing `menuItems`: mono 11.5px rows, selected item bg accent-tint + `var(--color-accent-200)` text, others `color-mix(in srgb,var(--color-text) 80%,transparent)`. Picking an item fires `onPick(key)` (caller closes).

7. **SegmentedControl.** Nocturne `.seg` container + one `.seg-opt` per option with a visually-hidden radio input (nocturne `.seg-opt input`), so the checked ring comes from nocturne's `.seg-opt:has(input:checked)` (accent text + inset 1px accent ring). Mono 11.5px, option padding `4px 10px` per mockup. Clicking (or checking via keyboard) an unselected option fires `onChange(value)` once; clicking the selected option fires nothing.

8. **Input.** Wrapper `<span class="input">` (nocturne) containing `<input class="wminput">` (app.css). Wrapper is `display:flex;align-items:center`; padding via module css default `6px 10px`, overridable by `className`. `mono` is inherent to `.wminput` (mono 12px); prop `mono` exists for API symmetry and defaults true — `mono:false` swaps the inner font to `var(--font-body)`. Controlled: `value` + `onChange(next: string)` (string, not event).

9. **Combobox.** The profile-search pattern (mockup lines 71–86), shaped like U00 CONTRACTS §3-style single-purpose contracts: an `Input` (placeholder from props) plus, when `open`, a dropdown surface absolutely positioned below (`padding:5px;border-radius:9px;background:var(--color-surface);box-shadow:var(--shadow-md)`) listing `items`: row = mono 12px `label` left + mono 10px 42%-text `sub` right, `border-radius:6px;padding:7px 9px`, hover/selected bg accent-tint. Click fires `onPick(key)`. Typing fires `onQuery`; focus fires `onOpenChange(true)`. Keys: Enter picks the first item (no-op when empty); Escape fires `onOpenChange(false)`. Empty `items` while open shows `emptyText` (mono 11px, 40% text).

10. **Toggle.** A `.sw` span with inner `<i>` knob (geometry per D00 §6). `on` adds `.on`; `disabled` adds `.off` (0.4 opacity, `pointer-events:none`) AND suppresses the handler. Click fires `onToggle(!on)`. Semantics: `role="switch"`, `aria-checked`, Space/Enter toggles (U00 SPEC §2.6).

11. **Tag.** `<span class="tag tag-accent|tag-neutral|tag-outline">` with the mono compact style used everywhere in the mockup (`font-size` and `padding` overridable via `className`; module default matches the mockup's `font-size:8.5px;padding:0 5px` neutral badges and `9.5px;1px 7px` accent chips via the `size` prop).

12. **Toast.** `ToastProvider` renders children plus at most ONE toast: mono, `position:fixed;left:50%;bottom:34px;translateX(-50%)`, `padding:9px 14px;border-radius:8px;background:rgba(14,15,24,.92)`, `box-shadow:var(--shadow-md),inset 0 0 0 1px color-mix(in srgb,var(--color-accent) 35%,transparent)`, `font-size:11.5px;color:var(--color-accent-100)`, `animation:toastIn .16s ease-out`, `white-space:nowrap` (D00 §6 toast token; mockup line 806 + `say()`). `useToast().show(message)` replaces any visible toast and restarts a 2.6s timer (mockup `say()`); timer cleared on unmount. `useToast` outside the provider throws.

13. **Tooltip.** Hover-anchored (mouseenter/mouseleave on a wrapper span): surface `padding:4px 7px;border-radius:6px;background:var(--color-surface);box-shadow:var(--shadow-md);font-size:10px;white-space:nowrap`, mono, positioned centered above the anchor (mockup sparkbar tip, line 307). No delay, no portal; shown only while hovered.

14. **Menu.** Anchor-positioned popup list: surface `padding:5px;border-radius:8px;background:var(--color-surface);box-shadow:var(--shadow-md)`, column with 1px gaps; item rows `padding:7px 9px;border-radius:5px;font-size:12px`, normal items 85%-text, `dim` items 55%-text, `mono` items mono 11.5px; `separator` items render `height:1px;margin:3px 6px;background:var(--color-divider)` (mockup app menu, lines 231–238). `role=menu`/`menuitem`. Click fires `onPick(key)`; Escape or any pointerdown outside the surface fires `onClose`. Rendering/position offsets are the caller's (absolute within a relative parent, via `className`).

15. **Table.** Header row + body from a render prop. Header cells: mono, `font-size:9.5px;letter-spacing:.08em;uppercase`; sortable cells are pointers and append `'  ↓'` (desc) / `'  ↑'` (asc) to the active column's label, colouring it `var(--color-accent-300)` vs 42%-text (mockup `benchSortCols`, lines 415–417 + 1333–1341). Clicking a sortable header fires `onSort({key, dir})` with dir `'desc'` on first activation and toggled thereafter (desc→asc→desc). Column `width`/`align` map to the mockup's flex pattern (`flex:1` when no width, else `flex:none;width`). Table does NOT sort rows — `rows(sort)` render-prop returns the body.

16. **DragList.** Vertical reorder via `@dnd-kit/sortable` (U00 SPEC Decisions). Renders `items[].node` in order with a drag-handle slot (the six-dot grab glyph area, `cursor:grab`); drag activates from the handle only. On drop in a new position fires `onReorder(ids)` with the full new id order; a drop in the same position fires nothing. While dragging, the active row gets an accent-tint background (mockup `providerRows` `bg`).

17. **EmptyState.** Single muted line: `font-size:11px;color:color-mix(in srgb,var(--color-text) 42%,transparent);max-width:56ch;text-wrap:pretty` (the mockup's footnote/empty style). Renders `text` only.

18. **CoverageBar.** 4px track (`border-radius:2px;background:color-mix(in srgb,var(--color-text) 10%,transparent);overflow:hidden`) with fill `background:var(--color-accent-600)`, width `round(covered/total*100)%` (mockup grRows, line 482). `total<=0` renders 0-width fill.

19. **ProviderPips.** Row of 6px dots, 3px gap, `border-radius:50%`; `true` → `var(--color-accent-400)`, `false` → `color-mix(in srgb,var(--color-text) 14%,transparent)` (mockup harnessRows pips, lines 508–510, 1510).

20. **UsageMeter.** Per mockup hpRows meters (lines 555–558, 1107–1113): a column (gap 5px) with a label line — mono 9px uppercase `letter-spacing:.07em` 38%-text label, right-aligned value (`{percent}%`, or `—` when `percent` is null; letter-spacing 0, no transform) — over a 4px bar (track as CoverageBar's, radius 2px). Fill: `null` → width 0 and grey fill `color-mix(in srgb,var(--color-text) 20%,transparent)`; else width `percent%`, colour `var(--color-accent-500)`, or `var(--color-accent-300)` when `percent >= 70`. `hot` forces the accent-300 fill regardless of value (caller-driven emphasis).

21. **SnippetPreview.** Mono block `padding:11px 12px;border-radius:8px;background:color-mix(in srgb,var(--color-text) 6%,transparent);font-size:11px;line-height:1.7;color:color-mix(in srgb,var(--color-text) 62%,transparent)` rendering `text` with preserved newlines (mockup agentSnippet, line 797). When `copyable`, click fires `onCopy(text)` and the block is a pointer (clipboard + toast are the caller's).

## 3. Error behaviour

- Components never throw on empty/degenerate props: empty `items`/`menuItems`/`states`/`columns` render the container (Combobox shows `emptyText`); `CoverageBar` with `total<=0` and `UsageMeter` with `percent:null` render empty tracks; percent inputs are clamped to 0..100 for width computation only (displayed text is untouched).
- `useToast` outside `ToastProvider` throws `Error('useToast requires ToastProvider')` — the only throwing surface.
- Disabled/readonly variants never fire callbacks (U00 SPEC §2.5).

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Vendoring | nocturne.css byte-identical minus the @import line; diffable against the mockup copy | U00 SPEC §2.3; keeps upstream design-system sync trivial |
| app.css scope | Helmet rules only, verbatim; `html,body`/`a` rules excluded | Those two style the document, which apps own |
| Open state | SplitButton/Combobox/Menu are fully controlled (`open` + `onOpenChange`/`onClose`) | Popover/settings decide exclusivity between popups (mockup `closeMenus`) |
| Toast concurrency | Single toast, replace + restart timer | Mockup `say()` behaviour exactly |
| Table sorting | Header-only; rows via render prop, caller sorts | Mockup sorts in state (`benchRows`); keeps Table dumb and reusable |
| Drag lib | @dnd-kit sortable, handle-activated, vertical | U00 Decisions; mockup uses HTML5 DnD but @dnd-kit is the shipping choice |
| UsageMeter null | `—` text + 0-width grey fill | Mockup `p.on ? p[k]+'%' : '—'` (line 1110) |
| cx form | Strings-only varargs | No classnames dep (U00 CONTRACTS §2); mockup never needs object form |
| Hook event target | `window` listeners, not pointer capture | Byte-for-byte mockup `drag()` semantics; capture would change move delivery |

## 5. Out of scope

Weight/balance/complexity controls (U03, consume the hook); results components (U04); any data fetching, query wiring, or `EngineHost` usage (apps, U05+); Inter font bundling (app entries, S01); the settings/popover shells that position Menu/Combobox surfaces (U05/U07).
