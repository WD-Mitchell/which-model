---
kind: feature-spec
version: "1.0"
feature: U00-ui
project: which-model-desktop
---

# U00-ui — Shared Frontend

## 1. Purpose

Two workspace packages deliver everything visual so both hosts (desktop now, website later) stay thin: `@which-model/core` (`packages/core`) holds types, the `EngineHost` interface, event names, and a full in-memory `MockEngineHost`; `@which-model/ui` (`packages/ui`) holds the theme and every component. App-level view composition and state wiring live in `apps/desktop` and are specified by the U05–U14 features.

Inherits `specs/desktop/global/*`. The mockup `specs/desktop/mockup/demo.dc.html` is normative for geometry, copy, colours, and interaction; its inline styles are ported into co-located CSS modules using nocturne custom properties.

## 2. Behaviour

1. **packages/core.** Zero runtime dependencies. TypeScript `strict`. Exports: `types.ts` (D00 CONTRACTS §2 mirrors, snake_case keys), `events.ts`, `host.ts`, `mock.ts`, `index.ts` barrel. `MockEngineHost` implements every method statefully in memory (profiles CRUD, rank recompute, toggles) using fixture data adapted from the mockup's `MODELS`/`SCALE`/`EXTRA`/`GROUP_DEFS`/`ALL_BENCH` constants, so every UI feature is fully exercisable without Go.

2. **packages/ui.** Peer deps: `react`, `react-dom` only (plus `@dnd-kit/*` as a real dep for `DragList`). Every component: props-only (D00 §2.1c); no `EngineHost` imports; no data fetching; controlled inputs (value + onChange). One folder per component: `ComponentName/{ComponentName.tsx, ComponentName.module.css, ComponentName.test.tsx}`; barrel `src/index.ts`.

3. **Styling.** `src/theme/nocturne.css` is the vendored design system (byte-identical to `specs/desktop/mockup/nocturne.css`). `src/theme/app.css` ports the mockup's app-level rules verbatim: `.mono`, `.ib` (+`.off`), `.row`, `.sw` (+`.on`,`.off`), `.scroll`, `.wminput`, `.wmsel`, `@keyframes toastIn`. Components use CSS modules + these globals; inline `style` attributes are allowed ONLY for values computed from props (bar widths, knob offsets, heights). Fonts: nocturne's Google-Fonts `@import` is removed in the vendored copy (desktop is offline-capable); Inter is bundled via `@fontsource/inter` imported once by each app entry — the tokens' font stacks are unchanged.

4. **Interaction primitives.** Pointer-drag controls replicate the mockup's `drag()` helper exactly: on pointerdown capture the target rect, translate every pointermove to `f = clamp((clientX - rect.left) / rect.width, 0, 1)`, call the mapper (e.g. `round(f*5)`), remove listeners on pointerup, and fire the change only when the mapped value differs from current. Implemented once as `usePointerFraction(onFraction)` in `packages/ui/src/hooks/usePointerFraction.ts`.

5. **Testing.** Vitest + `@testing-library/react` + jsdom, run per package (`pnpm --filter <pkg> test`). Every component test asserts: renders with representative props; fires its callbacks with the exact expected values (drag tests dispatch synthetic PointerEvents); disabled/read-only variants do not fire.

6. **Accessibility floor.** Interactive spans from the mockup become semantic elements (`button`, `input`, `[role=menu]/[role=menuitem]`, `[role=slider]` with `aria-valuenow` on weight/balance/complexity controls); keyboard: Enter/Space activates buttons/toggles, arrows adjust sliders by one step, Escape closes menus/popups. Visuals stay identical to the mockup.

7. **State (apps only).** TanStack Query owns server state; query keys are specified per feature (U05+). All queries invalidate on host events: `config:changed` → profiles/providers/harnesses/favourites/rank/catalog-line; `catalog:changed` → groups/benchmarks/rank; `usage:updated` → usage/providers; `settings:changed` → settings (+ rank when holds changed). Ephemeral popover overrides live in one Zustand store (U06). Toasts via `packages/ui` ToastProvider.

## 3. Error behaviour

- Rejected host promises carry `ErrorDTO`; app-level mutation handlers toast `message`; queries render inline error states with a retry affordance (spec'd per page).
- Components never throw on empty data: each list component's contract names its empty state.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Component API style | Controlled, props-only, one folder per component | Reuse across desktop + website hosts; testable without hosts |
| Drag implementation | Single `usePointerFraction` hook replicating mockup `drag()` | One behaviour for weights/balance/complexity; matches mockup feel exactly |
| Fonts | Bundle Inter via @fontsource; strip Google @import from vendored CSS | Desktop must not depend on network at startup |
| Test stack | vitest + testing-library + jsdom | Vite-native, fast, per-package |
| dnd | @dnd-kit (sortable, vertical) inside DragList only | Only reorder surface; keeps other components dependency-free |
| A11y | Semantic elements + slider roles, mockup visuals unchanged | Mockup uses spans; shipping product should not |

## 5. Files (area-level)

| Path | Owner |
|---|---|
| `packages/core/**` | U01 |
| `packages/ui/src/theme/*`, `hooks/usePointerFraction.*`, primitives (Button, SplitButton, SegmentedControl, Input, Combobox, Toggle, Tag, Toast, Tooltip, Menu, Table, DragList, EmptyState, CoverageBar, ProviderPips, UsageMeter, SnippetPreview) | U02 |
| `packages/ui` WeightRow/BalanceSlider/ComplexityScale/WeightEditor | U03 |
| `packages/ui` RankCarousel/RankList/ModelResultCard/ProfileWeightSparkbar | U04 |
| `apps/desktop/src/popover/**` | U05 (landing/shell/footer), U06 (weights view) |
| `apps/desktop/src/settings/**` shell/nav/registry | U07; one page dir per U08–U14 |
| `apps/desktop/src/lib/{queries.ts,invalidate.ts,overrides.ts}` | U05 (queries/invalidate), U06 (overrides) |
