---
kind: feature-contracts
version: "1.0"
feature: U02-theme-primitives
project: which-model-desktop
---

# U02-theme-primitives — Contracts

## 1. Package and files

All under `packages/ui/src/`. Every component follows U00 CONTRACTS §2: `components/<Name>/{<Name>.tsx, <Name>.module.css, <Name>.test.tsx}` — the table lists the folder once.

| Path | Contents |
|---|---|
| `theme/nocturne.css` | vendored, byte-identical to `specs/desktop/mockup/nocturne.css` minus the Google-Fonts `@import` line |
| `theme/app.css` | helmet rules ported verbatim (§3) |
| `lib/cx.ts` | `cx` |
| `hooks/usePointerFraction.ts` | `usePointerFraction` (shape fixed by U00 CONTRACTS §3) |
| `components/Button/` | `Button` |
| `components/SplitButton/` | `SplitButton` |
| `components/SegmentedControl/` | `SegmentedControl` |
| `components/Input/` | `Input` |
| `components/Combobox/` | `Combobox` |
| `components/Toggle/` | `Toggle` |
| `components/Tag/` | `Tag` |
| `components/Toast/` | `ToastProvider`, `useToast` (`Toast.tsx` exports both) |
| `components/Tooltip/` | `Tooltip` |
| `components/Menu/` | `Menu` |
| `components/Table/` | `Table` |
| `components/DragList/` | `DragList` (only file importing `@dnd-kit/*`) |
| `components/EmptyState/` | `EmptyState` |
| `components/CoverageBar/` | `CoverageBar` |
| `components/ProviderPips/` | `ProviderPips` |
| `components/UsageMeter/` | `UsageMeter` |
| `components/SnippetPreview/` | `SnippetPreview` |
| `index.ts` | barrel: every component, both props types and functions, `cx`, `usePointerFraction` |

Import boundary: nothing here imports `@which-model/core`, `apps/*`, or generated bindings (D00 SPEC §2.1c). React is a peer dep; `@dnd-kit/core` + `@dnd-kit/sortable` are real deps used only by DragList.

## 2. Exported API (exact TS)

```ts
// lib/cx.ts — strings-only class join (SPEC §2.3)
export function cx(...args: Array<string | false | null | undefined>): string

// hooks/usePointerFraction.ts — mockup drag() semantics (SPEC §2.4; U00 §2.4/§3).
// Returned handler: attach as onPointerDown. Captures currentTarget rect on down,
// adds window pointermove/pointerup, clamps f to [0,1], fires onFraction on the
// down event and every move, removes listeners on pointerup and on unmount,
// calls e.preventDefault() + e.stopPropagation() on the down event.
export function usePointerFraction(
  onFraction: (f: number) => void,
): (e: React.PointerEvent<HTMLElement>) => void

export interface ButtonProps {
  variant: 'primary' | 'secondary' | 'ghost'
  size?: 'md' | 'sm'          // default 'md' (nocturne metrics); 'sm' = mockup compact
  disabled?: boolean
  onClick?: () => void
  className?: string          // merged last via cx
  children: React.ReactNode
}

export interface SplitButtonMenuItem { key: string; label: string; selected: boolean }
export interface SplitButtonProps {
  label: string               // e.g. "Launch in Claude Code"
  onMain: () => void
  menuItems: SplitButtonMenuItem[]
  onPick: (key: string) => void
  open: boolean               // controlled (SPEC §2.6)
  onOpenChange: (open: boolean) => void
}

export interface SegmentedOption { value: string; label: string }
export interface SegmentedControlProps {
  options: SegmentedOption[]
  value: string
  onChange: (value: string) => void   // fires only for a different value
  className?: string
}

export interface InputProps {
  value: string
  onChange: (value: string) => void   // string, not the event
  placeholder?: string
  mono?: boolean                      // default true (.wminput); false → font-body
  disabled?: boolean
  className?: string                  // on the .input wrapper
  onFocus?: () => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLInputElement>) => void
}

export interface ComboboxItem { key: string; label: string; sub: string }
export interface ComboboxProps {
  items: ComboboxItem[]
  query: string
  onQuery: (query: string) => void
  open: boolean
  onOpenChange: (open: boolean) => void   // focus → true; Escape → false
  onPick: (key: string) => void           // click a row, or Enter → items[0]
  emptyText: string                       // shown when open && items.length === 0
  placeholder?: string
  selectedKey?: string                    // row highlighted accent (SPEC §2.9)
}

export interface ToggleProps {
  on: boolean
  disabled?: boolean          // .sw.off + handler suppressed
  onToggle: (on: boolean) => void   // called with !on
}

export interface TagProps {
  variant: 'accent' | 'neutral' | 'outline'
  size?: 'badge' | 'chip'     // badge = 8.5px/0 5px, chip = 9.5px/1px 7px (SPEC §2.11)
  onClick?: () => void
  className?: string
  children: React.ReactNode
}

export function ToastProvider(props: { children: React.ReactNode }): JSX.Element
export interface ToastHandle { show: (message: string) => void }  // 2.6s, single, replace
export function useToast(): ToastHandle    // throws outside ToastProvider (SPEC §3)

export interface TooltipProps {
  content: React.ReactNode    // e.g. "software_engineering  4 / 5"
  children: React.ReactNode   // hover anchor
}

export interface MenuItem {
  key: string
  label?: string              // required unless separator
  separator?: boolean
  dim?: boolean               // 55%-text (e.g. "Quit which-model")
  mono?: boolean              // mono 11.5px rows (harness picker)
  selected?: boolean          // accent bg + accent-200 text
}
export interface MenuProps {
  items: MenuItem[]
  onPick: (key: string) => void
  onClose: () => void         // Escape or outside pointerdown
  className?: string          // caller positions the surface
}

export interface TableColumn {
  key: string
  label: string
  width?: string              // e.g. '124px'; absent → flex:1
  align?: 'left' | 'center' | 'right'   // default 'left'
  sortable?: boolean
}
export interface TableSort { key: string; dir: 'asc' | 'desc' }
export interface TableProps {
  columns: TableColumn[]
  sort: TableSort | null
  onSort: (sort: TableSort) => void  // first activation desc, then toggle (SPEC §2.15)
  rows: (sort: TableSort | null) => React.ReactNode  // body render-prop; caller sorts
  className?: string
}

export interface DragListItem { id: string; node: React.ReactNode }
export interface DragListProps {
  items: DragListItem[]
  onReorder: (ids: string[]) => void  // full new order; unchanged drop fires nothing
  handle?: React.ReactNode            // drag-handle slot; default six-dot glyph
}

export interface EmptyStateProps { text: string }

export interface CoverageBarProps {
  covered: number
  total: number               // fill = round(covered/total*100)%; total<=0 → 0
  className?: string          // width comes from the caller (56px in group rows)
}

export interface ProviderPipsProps { states: boolean[] }

export interface UsageMeterProps {
  label: string               // rendered uppercase 9px mono
  percent: number | null      // null → "—" text, 0-width grey fill (SPEC §2.20)
  hot?: boolean               // force accent-300 fill; else auto at percent >= 70
}

export interface SnippetPreviewProps {
  text: string                // newlines preserved
  copyable?: boolean
  onCopy?: (text: string) => void   // fired on click when copyable
}
```

## 3. theme/app.css — closed rule list (ported verbatim from the mockup helmet `<style>`, demo.dc.html lines 13–28)

| Selector(s) | Source line |
|---|---|
| `.mono` | 13 |
| `.ib`, `.ib:hover`, `.ib.off, .ib.off:hover` | 14–16 |
| `.row:hover` | 17 |
| `.sw`, `.sw i`, `.sw.on`, `.sw.on i`, `.sw.off` | 18–22 |
| `.scroll` | 23 |
| `@keyframes toastIn` | 24 |
| `input.wminput`, `input.wminput::placeholder` | 25–26 |
| `select.wmsel`, `select.wmsel option` | 27–28 |

Excluded (owned by app entries): helmet lines 11 (`html,body`) and 12 (`a`). No additions.

## 4. Config keys / error codes / events

None. Errors: the single `useToast` throw (SPEC §3). No `ErrorDTO` handling at this layer.

## 5. Test fixtures (vitest + @testing-library/react + jsdom; one `<Name>.test.tsx` per folder)

| Component | Required assertions |
|---|---|
| cx | `cx('a', false, 'b', undefined) === 'a b'`; `cx() === ''` |
| usePointerFraction | Synthetic PointerEvent test: mount a target with a mocked `getBoundingClientRect` (`left:100,width:200`); dispatch pointerdown at clientX 150 → `onFraction(0.25)`; window pointermove at 400 → `onFraction(1)` (clamped); after pointerup, further moves fire nothing; down event received `preventDefault` + `stopPropagation`; unmount mid-drag removes window listeners |
| Button | classes per variant; `disabled` blocks `onClick` |
| SplitButton | main segment → `onMain`; chevron → `onOpenChange(true)`; with `open`, item click → `onPick(key)`; selected item carries the accent class |
| SegmentedControl | renders one `.seg-opt` per option with hidden radio; clicking unselected → `onChange(value)` once; clicking selected → no call; checked option's input is `checked` (ring via nocturne `:has`) |
| Input | typing → `onChange('text')`; placeholder rendered; `mono:false` drops the mono font class |
| Combobox | rows show label + sub; click → `onPick`; Enter → `onPick(items[0].key)`; Enter with empty items → no call; Escape → `onOpenChange(false)`; open + empty → `emptyText` visible; focus → `onOpenChange(true)` |
| Toggle | `.on` class from `on`; click → `onToggle(!on)`; `disabled` → `.off` class and no call; `role=switch` + `aria-checked`; Space fires |
| Tag | variant class mapping for all three |
| Toast | `show('x')` renders one toast; second `show` replaces (still one node); auto-dismiss after 2.6s (fake timers); `useToast` outside provider throws |
| Tooltip | hidden by default; mouseenter shows `content`; mouseleave hides |
| Menu | items render; separator renders divider not a menuitem; click → `onPick`; Escape → `onClose`; pointerdown outside → `onClose`; inside → not |
| Table | sortable header click with `sort:null` → `onSort({key,dir:'desc'})`; active column shows `↓`, toggling shows `↑`; non-sortable header click fires nothing; `rows` called with the `sort` prop |
| DragList | @dnd-kit reorder (keyboard-sortable simulation acceptable): moving id 0 below id 1 → `onReorder` with the full new order; no-move → no call |
| EmptyState | renders text |
| CoverageBar | `covered:3,total:4` → fill width `75%`; `total:0` → `0%` |
| ProviderPips | `[true,false]` → 2 dots, first accent-400 class, second neutral |
| UsageMeter | `percent:62` → text `62%`, fill accent-500 width `62%`; `percent:70` → accent-300; `percent:null` → text `—`, width `0%`, grey fill; `hot` with low percent → accent-300 |
| SnippetPreview | text rendered with newline preserved; `copyable` click → `onCopy(text)`; non-copyable click → no call |

Verify: `pnpm --filter @which-model/ui test` green; `pnpm --filter @which-model/ui build` emits `dist/` with both theme CSS files; `diff <(tail -n +3 specs/desktop/mockup/nocturne.css) <(tail -n +2 packages/ui/src/theme/nocturne.css)` empty apart from the removed `@import` (i.e. vendored file = mockup file with line 2 deleted).


## Correction — Use-case selector accessibility (2026-09-05)

Combobox rows are native buttons so every result can be selected with Tab and
Enter/Space. The popup scrolls within a 190px maximum height; long default or
all-use-case lists stay reachable without being clipped by the popover.
