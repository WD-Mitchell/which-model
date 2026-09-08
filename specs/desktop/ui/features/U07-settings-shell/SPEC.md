---
kind: feature-spec
version: "1.0"
feature: U07-settings-shell
project: which-model-desktop
---

# U07-settings-shell — Settings Window Shell

## 1. Purpose

The settings window's application frame in `apps/desktop/src/settings`: `SettingsApp` (entry component holding navigation state), `SettingsShell` (titlebar + sidebar + content column), `DetailHeader` (shared page/detail header), and the typed page registry (`pages.ts`) that U08–U15 plug their page components into. U07 owns WHICH page/detail is showing and the chrome around it; the pages themselves own their content. The mockup `specs/desktop/mockup/demo.dc.html` (settings window markup, lines 243–276; nav/meta constants, lines 938–956; back logic, line 1325) is normative.

Inherits `specs/desktop/global/*` and `specs/desktop/ui/*`. Depends on: U01 (types, `EngineHost`), U02 (ToastProvider, theme).

## 2. Behaviour

1. **Navigation state.** `SettingsApp` holds `{page: SettingsPageName, detail: Detail | null}`. `page` is one of the nav item names (CONTRACTS §3); initial page is `'Profiles'`. `Detail` is the tagged union in CONTRACTS §2 (`kind` ∈ profile/group/benchmark/provider/harness + `id`; the benchmark variant additionally carries `fromGroup` — the group-detail slug it was opened from, or `null`). Selecting any sidebar item sets `page` AND clears `detail` to `null`, even when re-selecting the current page (mockup `go()` always resets detail state).

2. **Back semantics.** `closeDetail()` mirrors the mockup's `onPageBack` exactly: when `detail.kind === 'benchmark'` and `fromGroup` is non-null, back returns to the originating group detail (`detail := {kind:'group', id: fromGroup}`); in every other case (including benchmark with `fromGroup: null`) `detail := null`, returning to the page's list view. `openDetail(d)` replaces `detail` wholesale; U09 passes `fromGroup` when opening a benchmark from a group detail.

3. **Titlebar.** One row (padding 11px 14px, bottom divider): a 10×10 close dot `#e05c55` that calls `host.window.closeSettings()` (D00 CONTRACTS §5); two inert 10×10 dots at `text 18%`; centred title `which-model — Settings` (heading font, 500, 12.5px, `text 72%`); 46px right spacer balancing the traffic lights. The titlebar div carries inline style `--wails-draggable: drag` and the close dot carries `--wails-draggable: no-drag`, making the web titlebar the native drag region per S00 (`specs/desktop/shell/SPEC.md` §2.3, `MacTitleBarHiddenInset`) — the mockup's JS `onWinDrag` is NOT ported.

4. **Sidebar.** 184px fixed column, background `text 3%`, right divider, padding 14px 10px. Renders the `NAV_GROUPS` constant (CONTRACTS §3): per group a 9px uppercase mono label (letter-spacing .13em, `text 34%`), then one row per item — 4×4 dot + 12.5px name. The active item (matching `page`) gets accent dot, accent-tinted background, and a 1px accent ring; inactive dots are `text 25%`-class transparent per the mockup. Footer (pinned via `margin-top:auto`): the config path in 9.5px mono `text 38%`, read from `settings.get().config_path` (query key `['settings']`); while loading or on error it renders empty — never the mockup's hard-coded `~/.config/which-model/config.toml`.

5. **Content column.** `flex:1; min-width:0`, class `.scroll`, column flex. `SettingsShell` mounts the active page component from `PAGE_REGISTRY` (CONTRACTS §4) via `React.lazy` inside a `<Suspense fallback={null}>`, passing `PageComponentProps` (CONTRACTS §2). Non-active pages unmount (no keep-alive); each page re-reads its queries on mount.

6. **DetailHeader.** Pages render exactly one `DetailHeader` at the top of the content column (padding 15px 22px 14px). Props: optional back link (chevron SVG + `backLabel`, 11.5px, `text 52%`, hover `text 7%` background — rendered only when `backLabel` is set, firing `onBack`); `title` 19px heading (500, letter-spacing −.015em); `blurb` 12px `text 55%`, `max-width:56ch`; optional right-aligned primary action button (12px, per `action.label`/`action.onAction`). In list state pages source title/blurb/action label from `PAGE_META` (CONTRACTS §5, verbatim mockup copy); in detail state they supply their own title/blurb (per U08–U14) with `backLabel` = the current page name (mockup: `backLabel: st.page` — the benchmark detail's back link is ALSO labelled `Benchmark groups` even though it returns to the group detail) and `onBack` = `closeDetail`.

7. **App root.** `main-settings.tsx` (the `settings.html` Vite entry) mounts `<ToastProvider>` → `<QueryClientProvider>` → `<SettingsApp host={...}/>`, resolving the host (MockEngineHost vs wailsHost) by the S04 mechanism. `SettingsApp` receives the host as a prop and provides it to pages via the app's host context/hook; U07 components make no direct binding imports (D00 §2.1e applies to `apps/desktop` as a whole, satisfied here).

## 3. Error behaviour

- `closeSettings()` rejection is toasted (`ErrorDTO.message`) — the window simply stays open.
- `['settings']` query failure only blanks the sidebar footer; navigation is pure client state and cannot fail.
- A `page` value with no registry entry is impossible by typing (`Record<SettingsPageName, …>` is total); no runtime fallback is specified.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Detail state shape | Single tagged union slot, not per-page slots | Mockup uses 5 parallel state keys (`bPage/gPage/pfPage/provPage/hPage`); one slot + `fromGroup` encodes the only legal combination (benchmark over group) without impossible states |
| Benchmark back target | `fromGroup` recorded at open time | Mockup keeps `gPage` set beneath `bPage` so back lands on the group; explicit field over implicit stacking |
| Back label | Always the page name | Mockup line 1324 (`backLabel: st.page`) — even for benchmark→group |
| Window drag | `--wails-draggable` styles, no JS drag | S00 §2.3: native drag region; mockup's `onWinDrag` is a browser-demo artifact |
| Header ownership | Pages render `DetailHeader`; shell renders only chrome | Detail titles/blurbs/actions need page data and handlers; keeps `PageComponentProps` minimal |
| Page mount policy | Lazy + unmount on switch | Registry stays a static typed table; TanStack Query caches make remounts cheap |
| Host delivery | Prop into `SettingsApp` from `main-settings.tsx` | Testable with `createMockEngineHost`; binding-swap stays in the entry file (S04) |

## 5. Out of scope

- Page content, list rows, detail bodies, page actions' effects — U08–U14 (they receive `openDetail`/`closeDetail` and the `PAGE_META` action label only).
- Window creation, sizing (820×520), show/hide lifecycle — S03; `closeSettings` binding — S04.
- Popover shell — U05.


## Correction (2026-09-05)

The Profiles / Use Cases correction in `specs/desktop/backend/features/B03-profiles/SPEC.md` governs the new persisted profile selection and desktop terminology. The DTO extension is canonical in `specs/desktop/global/CONTRACTS.md`. Settings navigation now has both Profiles (curated defaults) and Use Cases (ranking presets).
