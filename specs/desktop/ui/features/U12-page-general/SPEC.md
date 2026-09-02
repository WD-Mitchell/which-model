---
kind: feature-spec
version: "1.0"
feature: U12-page-general
project: which-model-desktop
---

# U12-page-general — General Settings Page

## 1. Purpose

`GeneralPage` (`apps/desktop/src/settings/pages/general/`) is the "General" page inside the U07 settings shell: global shortcut, six system toggles, update frequency, and the three results-display preferences (ranking layout, weight control style, ranks held). Every control reads the single `GUISettings` struct and writes it back whole with exactly one field changed.

Depends on: U02, U07. Inherits `specs/desktop/global/*` (D00) and `specs/desktop/ui/*` (U00). Mockup `onGeneral` block of `specs/desktop/mockup/demo.dc.html` is normative for geometry and copy.

## 2. Behaviour

1. **Data binding.** The page reads query `['settings']` (`host.settings.get()`). Every control writes via `host.settings.set({...current, <field>: <newValue>})` — the whole struct with exactly one field changed. Each control has its own handler; there is NO debounce (all controls are discrete). The resulting `settings:changed` event invalidates `['settings']` per U00 CONTRACTS §5, refreshing the page.

2. **"system" section.** Uppercase mono section label `system` (accent, 9px, .13em tracking, per mockup). First row: title "Open the popover", note "Global shortcut, works from any app.", and a mono `SegmentedControl` (U02) on the right with display options `⌥ Space` / `⌃ Space` / `⇧⌘ M`. The seg maps display glyphs ↔ canonical `GUISettings.Shortcut` strings (D00 CONTRACTS §2): `⌥ Space` ↔ `alt+space`, `⌃ Space` ↔ `ctrl+space`, `⇧⌘ M` ↔ `cmd+shift+m`. Selecting an option writes `shortcut`.

3. **Toggle grid.** Below the shortcut row, a 2-column grid (`1fr 1fr`, 28px column gap) of six `Toggle` (U02) rows, names verbatim and bound to the listed `GUISettings` fields:
   | Name | Field |
   |---|---|
   | Show menu bar icon | `show_menu_bar_icon` |
   | Launch at startup | `launch_at_login` |
   | Store sign-ins in system keychain | `use_keychain` |
   | Copy launch command instead | `copy_command_instead` |
   | Close popover after launching | `close_popover_after_launch` |
   | Install updates automatically | `auto_update` |
   Row label colour is bright when on, muted when off (mockup `g.fg`).

4. **Check for updates.** Next grid cell: label "Check for updates" and a `.wmsel` native `<select>` with options Hourly/Daily/Weekly/Monthly (values `hourly`/`daily`/`weekly`/`monthly`) bound to `auto_update_frequency`. When `auto_update` is false the WHOLE row is dimmed to opacity `.38`; per the mockup the select stays operable while dimmed.

5. **"results display" section.** Second uppercase label `results display`. Three rows:
   a. **Ranking layout** — note "How the popover presents the held ranks." Right side: two `LayoutSwatch` mini-mockups (82×40, radius 6) captioned `carousel` and `list`, drawn as in the mockup (carousel: two 4×7 side nubs + centred accent bar + 70% grey bar; list: three stacked bars — accent, 80% grey, 62% grey). Clicking one writes `layout`.
   b. **Weight control** — note "Used for every profile weight and the scale." Three `ControlSwatch` mini-mockups (66×26) captioned `step` (four 9×8 cells, first two accent), `bar` (7px track, 62% accent fill), `slider` (2px track + accent knob at 58%). Clicking writes `weight_control`.
   c. **Ranks held per pick** — mono seg with options `3` / `5` / `10` writing `holds` (number).

6. **Swatch selection state** (both pickers, mockup `swatch()`): selected → ring `inset 0 0 0 1.5px var(--color-accent)`, opacity 1, caption `var(--color-accent-200)`; unselected → ring `inset 0 0 0 1px color-mix(in srgb,var(--color-text) 10%,transparent)`, opacity .55, muted caption. Clicking the already-selected swatch fires nothing.

7. **Page chrome.** Title/blurb are supplied to the U07 registry: title "General", blurb "How which-model runs on this Mac, and how the pick is drawn in the popover.", no page action.

8. **"catalogue" section.** Uppercase mono label `catalogue`. Rows:
   a. **Data source repo** — note "Benchmarks are pulled from this GitHub repository. Default is the main which-model repo." Right side: text field bound to `catalog_repo` (`owner/repo` or `owner/repo@ref`). Empty blurs to `WD-Mitchell/which-model`. Commit on blur.
   b. **Collect locally** — note "Optional. Use your own Artificial Analysis API key instead of the repo." Toggle bound to `use_local_aa`. Off (default) pulls `catalog_repo`; on runs `catalog refresh` with a local AA key.
   c. **Artificial Analysis API key** — shown only when `use_local_aa` is on. Password field; Get never echoes the key (`aa_api_key_set` indicates a saved key). Commit on blur. Sentinel `-` clears the sidecar file. The key is NOT written to config.toml.

## 3. Error behaviour

- `['settings']` pending → nothing rendered (page body blank until data); error → inline error state with retry (U00 SPEC §3).
- A rejected `settings.set` toasts `ErrorDTO.message`; the control re-renders from the (unchanged) query cache — no local optimistic state is kept.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Write granularity | Whole struct, one field per handler, no debounce | All controls discrete; matches `settings.set(GUISettings)` contract; trivial to test |
| Shortcut representation | Display glyphs mapped to D00 canonical strings | `GUISettings.Shortcut` enum is `alt+space`/`ctrl+space`/`cmd+shift+m` (D00 CONTRACTS §2) |
| Dimmed select stays enabled | opacity .38 only, still operable | Mockup keeps `onUpdateFreq` wired regardless of `autoupdate` |
| Swatch pickers | Two local presentational subcomponents, not `packages/ui` | Only this page draws mini-mockups; keeps ui package free of one-off art |
| Selected-state no-op | Re-clicking selected seg option/swatch fires no `set` | SegmentedControl contract (U02); avoids redundant writes |
