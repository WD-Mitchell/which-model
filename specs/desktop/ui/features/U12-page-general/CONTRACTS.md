---
kind: feature-contracts
version: "1.0"
feature: U12-page-general
project: which-model-desktop
---

# U12-page-general — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `apps/desktop/src/settings/pages/general/GeneralPage.tsx` | `GeneralPage` + registry entry export |
| `apps/desktop/src/settings/pages/general/LayoutSwatch.tsx` | local `LayoutSwatch` |
| `apps/desktop/src/settings/pages/general/ControlSwatch.tsx` | local `ControlSwatch` |
| `apps/desktop/src/settings/pages/general/GeneralPage.module.css` | grid/section/swatch styles |
| `apps/desktop/src/settings/pages/general/GeneralPage.test.tsx` | fixtures §5 |

## 2. Exports

```ts
// GeneralPage: no props; obtains EngineHost via the app host context (U07)
// and TanStack Query key ['settings'].
export function GeneralPage(): JSX.Element

// LayoutSwatch/ControlSwatch: pure presentational; selection visuals per SPEC §2.6.
export interface LayoutSwatchProps {
  kind: 'carousel' | 'list'      // which 82×40 mini-mockup to draw; caption = kind
  selected: boolean
  onSelect: () => void            // NOT called when selected
}
export function LayoutSwatch(props: LayoutSwatchProps): JSX.Element

export interface ControlSwatchProps {
  kind: 'step' | 'bar' | 'slider' // which 66×26 mini-mockup to draw; caption = kind
  selected: boolean
  onSelect: () => void            // NOT called when selected
}
export function ControlSwatch(props: ControlSwatchProps): JSX.Element
```

## 3. Copy (verbatim)

| Slot | Text |
|---|---|
| Section labels | `system`, `results display` |
| Shortcut row | `Open the popover` / `Global shortcut, works from any app.` |
| Shortcut seg options | `⌥ Space`, `⌃ Space`, `⇧⌘ M` |
| Toggles | `Show menu bar icon`, `Launch at startup`, `Copy launch command instead`, `Close popover after launching`, `Install updates automatically` |
| Select row | `Check for updates`; options `Hourly`, `Daily`, `Weekly`, `Monthly` |
| Layout row | `Ranking layout` / `How the popover presents the held ranks.`; captions `carousel`, `list` |
| Control row | `Weight control` / `Used for every profile weight and the scale.`; captions `step`, `bar`, `slider` |
| Holds row | `Ranks held per pick`; seg options `3`, `5`, `10` |

## 4. Shortcut mapping (closed)

| Display | `GUISettings.shortcut` |
|---|---|
| `⌥ Space` | `alt+space` |
| `⌃ Space` | `ctrl+space` |
| `⇧⌘ M` | `cmd+shift+m` |

## 5. Test fixtures (`GeneralPage.test.tsx`, vitest + testing-library + `createMockEngineHost`)

- **One-delta writes.** For EACH control (shortcut seg option, each of the 5 toggles, select change, each layout swatch, each control swatch, each holds option): activating it calls `settings.set` exactly once with the full current `GUISettings` in which ONLY that control's field differs (deep-equal on every other field).
- Shortcut: clicking `⇧⌘ M` writes `shortcut: 'cmd+shift+m'` (glyph→canonical).
- Holds: clicking `10` writes `holds: 10` (number, not string).
- **Select dimming.** With `auto_update: false` the Check-for-updates row has opacity `.38`; with `true`, opacity `1`; changing the select still fires while dimmed.
- **No redundant writes.** Clicking the selected seg option / selected swatch fires no `set`.
- Swatch visuals: selected swatch has the 1.5px accent ring and opacity 1; unselected has opacity .55.
