---
kind: feature-contracts
version: "1.0"
feature: U05-popover-landing
project: which-model-desktop
---

# U05-popover-landing — Contracts

## 1. Files

| File | Contents |
|---|---|
| `apps/desktop/src/popover/PopoverApp.tsx` + `.css` + `.test.tsx` | view state machine, selectedIndex/stop/harnessSlug state, launch flow |
| `apps/desktop/src/popover/PopoverShell.tsx` + `.css` + `.test.tsx` | 400px chrome: arrow, drop-shadow, glow, header row, app `Menu` |
| `apps/desktop/src/popover/LandingView.tsx` + `.css` + `.test.tsx` | hero + catalog line, combobox, `or slide` + `ComplexityScale`, profile name |
| `apps/desktop/src/popover/PopoverFooter.tsx` + `.css` + `.test.tsx` | footer row, landing variant content, harness menu; weights variant slot (content from U06) |
| `apps/desktop/src/lib/queries.ts` + `queries.test.ts` | typed hooks per U00 §6 (SPEC §2.11) |
| `apps/desktop/src/lib/invalidate.ts` + `invalidate.test.ts` | `useEngineEvents()` implementing U00 §5 map |
| `apps/desktop/src/main-popover.tsx` | popover entry: QueryClientProvider + ToastProvider + `useEngineEvents()` + `<PopoverApp/>` |

DTO/host types from `@which-model/core` (D00 §2/§5); components from `@which-model/ui`. No `EngineHost` access outside `lib/` hooks and mutation handlers.

## 2. Props / API

```ts
export type PopoverView = 'landing' | 'weights'

export interface PopoverShellProps {
  header: React.ReactNode          // landing brand row or U06 weights header
  menuOpen: boolean
  onToggleMenu(): void
  onCustomWeights(): void          // app menu items (SPEC §2.3)
  onOpenSettings(): void
  onQuit(): void
  children: React.ReactNode
}

export interface LandingViewProps {
  catalog?: CatalogSummary
  profiles: ProfileSummary[]
  scale: string[]                  // complexityScale() slugs
  activeSlug: string
  activeName: string
  stop: number                     // 0..4, sticky (SPEC §2.6)
  onSelectProfile(slug: string, scaleIndex: number | null): void
}

export interface PopoverFooterProps {
  variant: PopoverView
  // landing:
  harnesses?: HarnessInfo[]
  harnessSlug?: string
  harnessMenuOpen?: boolean
  onToggleHarnessMenu?(): void
  onPickHarness?(slug: string): void
  onManage?(): void
  onLaunch?(): void
  children?: React.ReactNode       // weights variant buttons (U06)
}

// lib/queries.ts (shapes; full hook list in SPEC §2.11)
export function useRank(slug: string, overridesHash: string, holds: number):
  UseQueryResult<RankResponse>     // key ['rank', slug, overridesHash, holds]
export function overridesHashOf(o: ProfileDetail | null): string  // stable stringify | 'none'

// lib/invalidate.ts
export function useEngineEvents(): void   // subscribes on mount, disposes on unmount
```

## 3. Copy strings (exact)

| Where | String |
|---|---|
| Brand | `which-model` |
| Hero title | `The right model for the job in front of you.` |
| Catalog line | `{models} models · {providers_on} providers on · {harnesses} harnesses` |
| Search placeholder | `type to find a profile` |
| No matches | `no profile by that name` |
| Combobox sub | `{core_share}/{100-core_share}` |
| Scale label / ends | `or slide` · `simple action` · `planning` |
| Carousel | `rank {n} of {total}` · meta `{provider} · {reasoning} · {score2dp}` |
| Carousel empty | `no route` · `Enable a provider` · `every provider is switched off` |
| Footer | `Manage profiles` · `Launch in {harnessName}` |
| No-pick launch toast | `no model to launch — enable a provider` |
| Launch toast | the substituted `LaunchResult.command` |
| App menu | `Custom weights…` · `Settings…` · `Quit which-model` |

## 4. Geometry not in D00 §6

Harness menu width 158px (right 16, bottom 44); app menu width 176px (right 14, top 36); search popup inset 18px, top 44, 5 rows max; glow 210×210 at `inset:-70px -60px auto auto`; arrow right margin 96px.

## 5. Test fixtures (vitest + `createMockEngineHost()`)

| Test | Assertion |
|---|---|
| profile pick updates rank | pick a combobox item → `useRank` refetches with new slug, `selectedIndex` back to 0, overrides store cleared |
| sticky stop | pick an off-scale profile → handle stays at previous stop; pick scale profile → handle moves to its index |
| launch (spawn mode) | mock `harnesses.launch` → `{copied:false, command}` → toast shows command; `copyToClipboard` NOT called |
| launch (copy mode) | `{copied:true, command}` → `window.copyToClipboard(command)` called once, toast shows command |
| launch no pick | zero candidates → toast `no model to launch — enable a provider`; `launch` never called |
| close after launch | `close_popover_after_launch:true` → `window.hidePopover()` called; false → not called |
| harness memory | pick harness in menu → label updates; `settings.set` / `harnesses.save` never called |
| event invalidation smoke | fire `config:changed` on mock host → `['profiles']` and `['rank', …]` queries refetch; `settings:changed` → `['settings']` |
