---
kind: feature-spec
version: "1.0"
feature: U11-page-harnesses
project: which-model-desktop
---

# U11-page-harnesses — Harnesses Settings Page

## 1. Purpose

The Harnesses page of the settings window: the list of detected + custom harnesses (launch command, per-harness provider pips, remove, "Add custom"), and a per-harness detail with the launch-command template, per-provider allow toggles, and live usage meters. It is the GUI for `[harness.*]` config. Lives in `apps/desktop/src/settings/pages/harnesses/`, registered in U07's page registry under `Harnesses`; visuals are normative from the mockup (`specs/desktop/mockup/demo.dc.html`, list ~lines 492–522, detail ~524–572).

Depends on: U02 (Toggle, Button, Input, Tag, UsageMeter, useToast), U07 (shell, `DetailHeader`, registry, `PageComponentProps`).

## 2. Behaviour

1. **Data.** Three queries: `['harnesses']` → `harnesses.list()`, `['providers']` → `providers.list()` (row order, global enabled/auth), `['usage', false]` → `usage.snapshots(false)` (meters). Detail rows join `UsageDTO` to providers by `UsageDTO.provider === ProviderInfo.id`; a provider with no snapshot renders all three meters as unknown (`—`, empty bar).

2. **List.** U07 header shows PAGE_META copy with page action `Add custom`. Section label `harnesses`; column header row: `harness` 120px / `providers` 84px / `launch command` flex / 44px actions spacer. One row per `HarnessInfo` in list order: name (ellipsis) + neutral `custom` tag when `!builtin`; a pip per provider (global priority order — accent when `providers[id]` is true, dim otherwise) + count `{n} of {providers.length}` or the literal `none` when zero; the command template (mono, ellipsis); a trash icon on EVERY row (builtins removable too, title `Remove {name}`); chevron. Row click opens detail; trash click stops propagation. Below the rows, the footnote verbatim: `harnesses and their providers are read from each harness’ own config on launch`.

3. **Remove.** Trash calls `harnesses.delete(slug)` then toasts `removed {name}`. (The mockup silently removes; the toast is a deliberate addition matching this app's other destructive actions.)

4. **Add custom.** The page action creates a `HarnessInfo` with: `name` = `Custom N` where N = (count of current custom harnesses) + 1, incremented further while the name collides with any existing harness name; `slug` = name lower-cased, spaces → `_` (e.g. `custom_2`); `command` = `my-agent --model {model_id}`; `builtin: false`; `installed: false`; `providers` = map of every provider id → true iff that provider is globally enabled. It calls `harnesses.save(h)` and toasts `{name} added`. The list stays; no auto-open.

5. **Detail header.** `DetailHeader` back link `Harnesses`, title = harness name, blurb per CONTRACTS §4.

6. **Launch command.** Section `launch command` with the note `substituted at launch from the pick` beside it, then the command box. For a builtin harness it is a read-only mono box showing `command`. For a custom harness it is a U02 `Input` (same mono styling): edits update local state immediately and, debounced 300ms after the last keystroke, call `harnesses.save({...harness, command})`. Unmount/navigation flushes a pending save. Builtins never call `save` (`builtin_readonly` guard is client-side too).

7. **Providers section.** Header `providers` + summary `{n} of {providers.length} enabled` (n = providers whose map value is true) + ghost `Enable all` / `Disable all` calling `harnesses.setAllProviders(slug, true|false)`. Then one card row per provider in global priority order: `Toggle` (→ `harnesses.setProvider(slug, id, !on)`), provider id + neutral `detected` tag, auth line = `ProviderInfo.auth` when the provider is globally enabled else the literal `off globally`, three `UsageMeter`s, and a 138px right column with `credits` over `resets` (from the joined `UsageDTO`, falling back to `ProviderInfo.credits/resets` when no snapshot).

8. **Meter rules.** For each of `session`/`weekly`/`monthly` (matched to `UsageWindow.id`): bar width = used % when the provider is globally on, else 0. Fill when the row is on in this harness AND the provider is globally on: `--color-accent-500`, or `--color-accent-300` at ≥ 70%; otherwise fill `text 20%`. Right text = `{pct}%` when globally on else `—`; text/labels dim when either switch is off. Row card: on ⇒ bg `text 4%` + accent ring; off ⇒ transparent + faint ring. Credits bright only when the provider is globally on.

9. **Detected tag.** Shown on a row iff the harness is builtin and the host-reported `providers` map has that id `true` (detection is where a builtin's allow-map comes from). Custom harnesses never show it.

10. **Footnote.** Below the rows, verbatim: `A provider switched off here is never used when launching in this harness.`

## 3. Error behaviour

- Rejected mutations toast `ErrorDTO.message`; state re-syncs on refetch (`config:changed` invalidates `['harnesses']`, `usage:updated` invalidates `['usage']`/`['providers']`).
- `usage.snapshots` rejecting (`usage_unavailable`) is NOT an error state: rows render with unknown meters; no toast.
- Harness/provider list query errors render `couldn't load harnesses` + ghost `Retry`.
- Detail slug vanishing after refetch navigates back to the list.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Meter data source | `usage.snapshots(false)` joined by provider id; `ProviderInfo` for enabled/auth/order | snapshots are the canonical windowed numbers; ProviderInfo % fields serve the Providers list only |
| Command editing | custom-only, debounced 300ms, flush on unmount | builtin commands are read-only (`builtin_readonly`); debounce avoids a save per keystroke |
| Custom naming | `Custom N`, N = custom-count+1 then bump on collision | mockup line 1204; collision bump makes delete-then-add deterministic |
| New-custom providers | all globally-enabled providers on | mockup line 1206 |
| Remove allowed on builtins | yes, any row | mockup renders the trash unconditionally; detection re-adds on next scan |
| Delete toast | `removed {name}` | task addition; mockup silent — consistency with group/profile deletes |
| Detected tag source | builtin && `providers[id]` true from host | DTO carries no separate detection flag; the builtin allow-map IS the detection result |
| Off-globally rendering | width 0, text `—`, fill/text dimmed, auth `off globally` | mockup lines 1103–1116 |

## 5. Out of scope

- `UsageMeter`, `Toggle`, `Input`, `Tag`, toast — U02. Shell/header/registry — U07.
- Launching (`harnesses.launch`) — U05 popover footer.
- Usage fetching/backends and the Usage-detection page — U13.
- Go-side harness detection/persistence — backend features; `MockEngineHost` suffices.
