---
kind: feature-spec
version: "1.0"
feature: U05-popover-landing
project: which-model-desktop
---

# U05-popover-landing — Popover Shell, Landing View, App Wiring

## 1. Purpose

The popover is the product's front door: a 400px NSPopover-style panel where the user names the job (profile search or complexity slide), sees the ranked pick, and launches it into a harness. U05 owns the popover app shell (`apps/desktop/src/popover/`), the landing view, the shared footer, and the app-level data wiring every popover/settings feature reuses: typed TanStack Query hooks (`lib/queries.ts`) and the host-event → query-invalidation bridge (`lib/invalidate.ts`). The weights view body is U06; U05 mounts it.

Depends on: U01 (core/host/mock), U02 (primitives), U03 (ComplexityScale), U04 (RankCarousel/RankList). Mockup `specs/desktop/mockup/demo.dc.html` popover markup (lines 55–241) is normative for geometry, copy, and colour.

## 2. Behaviour

1. **Shell.** `PopoverShell` renders the mockup chrome exactly: outer wrapper 400px wide with `filter: drop-shadow(0 22px 50px rgba(0,0,0,.55))`; an 8px CSS-triangle arrow (`border-bottom: 8px solid var(--color-bg)`) right-aligned with 96px right margin; the panel (`background: var(--color-bg)`, radius 12px, `--shadow-lg`, `overflow: hidden`, font-size 13px); and the corner radial glow — an absolutely positioned 210×210 circle at `inset: -70px -60px auto auto`, `radial-gradient(circle, color-mix(in srgb, var(--color-accent) 20%, transparent), transparent 70%)`, `pointer-events: none`. Width/arrow/radius per D00 §6.

2. **View state machine.** `PopoverApp` holds `view: 'landing' | 'weights'` (initial `'landing'`), plus popover-scoped UI state: `selectedIndex` (rank carousel/list index, initial 0), `harnessSlug` (initial: first item of `harnesses.list()`), and open flags for the app menu, harness menu, and search popup. App menu "Custom weights…" → `view = 'weights'`; WeightsView back chevron → `'landing'`. A click anywhere inside the popover closes any open menu/popup first (mockup `onPopoverClick`).

3. **Header.** Landing header: brand "which-model" (`--font-heading`, 500, 12.5px, letter-spacing -.01em) + hamburger icon button (`.ib`, 20×20) right-aligned. The hamburger toggles the app `Menu` (176px, anchored top-right at right 14px / top 36px): `Custom weights…`, `Settings…` → `window.openSettings()`, divider, `Quit which-model` → `window.quit()`. The mockup's quit toast ("which-model quit — click the glyph to bring it back") is simulation-only chrome and is NOT shipped — the real process exits. The weights-view header (chevron + "Weights for {slug}" + same hamburger) is rendered by U06; the Menu component instance and its handlers are U05's and are passed down.

4. **Landing hero.** Title "The right model for the job in front of you." (`--font-heading`, 500, 23px, `max-width: 16ch`). Beneath it the catalog line (mono, 10.5px): `"{models} models · {providers_on} providers on · {harnesses} harnesses"` formatted from `CatalogSummary` (`useCatalogLine()`); real numbers, never the mockup's literal 412.

5. **Profile search combobox.** Input placeholder `type to find a profile`. Matching: case-insensitive substring of the query against profile `name` (lowercased) OR `slug`; empty query matches all; first 5 shown. Item: mono `slug` left, sub right = `"{core_share}/{100 - core_share}"` (the core/task split, e.g. `60/40`); the active profile's row gets the accent background/foreground. Popup opens on focus or typing; Enter picks the first match (no-op when none); Escape closes the popup; picking closes it and clears the query. Zero matches while open → single muted row `no profile by that name`.

6. **Complexity scale.** Label row `or slide` (10px uppercase). `ComplexityScale` (U03) with 5 stops at 0/25/50/75/100% (D00 §6), end labels `simple action` / `planning`, and the active profile name (15px, `--color-accent-300`) centred below. Stops map 1:1 onto `useComplexityScale()` (`profiles.complexityScale()`, 5 slugs, D00 order). Handle position: `stop = scale.indexOf(activeProfileSlug)`; when the active profile is NOT on the scale (picked via search), the handle KEEPS its last stop value — `stop` is separate `PopoverApp` state updated only when a pick resolves to a scale index (mockup keeps `st.stop`). Dragging maps fraction → `round(f*4)` and applies the profile at that scale index.

7. **Selecting a profile** (combobox pick, scale drag, or list-row pick does NOT change profile — rows change `selectedIndex` only): set active slug, reset `selectedIndex` to 0, clear + close search, update `stop` per §2.6, and reset the U06 overrides store to the newly active profile (`overrides.clear()` then init from it) — a profile change always discards ephemeral edits.

8. **Results band.** Layout comes from `useSettings().layout`; holds from `useSettings().holds`. Landing shows `RankCarousel` (U04) when `layout === 'carousel'`, `RankList` when `'list'`; the weights view always shows the carousel (mockup `showCarousel`). Data: `useRank(activeSlug, overridesHash, holds)` → `RankResponse`; `pick = candidates[min(selectedIndex, candidates.length - 1)]`. Carousel copy: `rank {n} of {total-held}`, name, meta `"{provider} · {reasoning} · {score.toFixed(2)}"`. Empty candidates: rank text `no route`, name `Enable a provider`, meta `every provider is switched off`; chevrons disabled at ends.

9. **Footer (landing variant).** Left: ghost button `Manage profiles` → `window.openSettings()` (Profiles page). Right: `SplitButton` — mono label `Launch in {harnessName}` + chevron segment opening the harness menu (158px, anchored right 16px / bottom 44px) listing `useHarnesses()` names; picking one sets `harnessSlug` and closes the menu. The selection lives ONLY in `PopoverApp` memory — it is never written to config/settings and reverts to the first listed harness on relaunch (mockup parity; deliberate).

10. **Launch flow.** On `Launch in {harness}`: if there is no pick → toast `no model to launch — enable a provider` and stop. Otherwise call `harnesses.launch(harnessSlug, pick.route_key, activeSlug)`. On resolve with `LaunchResult`: when `copied === true` → `window.copyToClipboard(result.command)` then toast the command; when `false` → toast the fully substituted `result.command` (mockup toasts the substituted template in both cases). Then, when `useSettings().close_popover_after_launch` is true → `window.hidePopover()`. Rejection → toast `ErrorDTO.message`. Pick counting is engine-side (`pick:recorded` event → invalidation), not frontend state.

11. **Queries (`lib/queries.ts`).** One typed hook per U00 §6 canonical key, thin wrappers over `getHost()` (U01): `useProfiles ['profiles']`, `useProfile(slug) ['profile', slug]`, `useComplexityScale ['complexity-scale']`, `useRank(slug, overridesHash, holds) ['rank', slug, overridesHash, holds]`, `useCatalogLine ['catalog-line']`, `useGroups ['groups']`, `useGroupDetail ['group', slug]`, `useBenchmarks ['benchmarks']`, `useBenchmarkDetail ['benchmark', name]`, `useProviders ['providers']`, `useProviderDetail ['provider', id]`, `useHarnesses ['harnesses']`, `useUsage(force) ['usage', force]`, `useFavourites ['favourites']`, `useSettings ['settings']`, `useSnippets ['snippets']`. `useRank` builds `RankRequest` with `overrides` from the U06 store when dirty, else omitted; `overridesHash` per U00 §6 (`'none'` when clean).

12. **Invalidation (`lib/invalidate.ts`).** `useEngineEvents()` — mounted once per app root (popover AND settings) — subscribes via `host.on` to every event and invalidates exactly the U00 §5 map (`['rank']` by prefix). Unsubscribes on unmount using the returned disposer.

## 3. Error behaviour

- Query errors render the mockup layout with em-dash placeholders (`—`) in the affected band and toast nothing; mutation/launch rejections toast `ErrorDTO.message` (U00 §3).
- Empty profiles list: combobox popup shows the no-match row; scale renders but drags are no-ops until the scale query resolves.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Harness selection persistence | In-memory only, default = first of `harnesses.list()` | Mockup parity; launch target is situational, not config |
| Off-scale profile handle | Keep last `stop`; never snap or hide | Mockup keeps `st.stop`; handle is a shortcut, not a mirror |
| Launch toast content | Always the substituted command (copied or spawned) | Mockup behaviour; command doubles as confirmation + recovery |
| Quit toast | Not shipped | Mockup-simulation chrome; real `window.quit()` exits |
| Combobox matching | Substring over lowercased name + slug, cap 5 | Mockup `matches` logic verbatim |
| Wiring location | `apps/desktop/src/lib`, not packages | U00 §2.7: apps own state; packages stay host-free |

## 5. Out of scope

- Weights view body, overrides store, save/copy footer — U06.
- Carousel/list/card internals — U04; scale/weight primitives — U03.
- Settings window and its pages — U07+. Tauri window plumbing behind `window.*` — backend features.


## Completed empty ranking correction — #175 review

When an enabled ranking query completes successfully with zero candidates,
clear the native tray pick and pending launch state. Loading and failed
requests preserve the last successful tray state. Do not publish results from
a disabled query or a superseded request. This rule governs the popover-to-tray
bridge as well as the S02 native host.

## Correction — Profiles and Use Cases (2026-09-05)

The user's requested distinction between Profiles and Use Cases supersedes the
conflicting terminology and Quick complexity-scale behavior above. The governing
behavior and pinned validation cases are in
`specs/desktop/backend/features/B03-profiles/SPEC.md` §Correction. Canonical DTOs
are extended in `specs/desktop/global/CONTRACTS.md` §Profiles / Use Cases extension.
