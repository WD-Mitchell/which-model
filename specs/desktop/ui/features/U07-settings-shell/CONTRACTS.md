---
kind: feature-contracts
version: "1.0"
feature: U07-settings-shell
project: which-model-desktop
---

# U07-settings-shell — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `apps/desktop/src/settings/SettingsApp.tsx` | `SettingsApp` — nav state, `openDetail`/`closeDetail`, host context provider |
| `apps/desktop/src/settings/SettingsApp.test.tsx` | §6 state/back/close tests |
| `apps/desktop/src/settings/SettingsShell.tsx` | titlebar + sidebar + content column chrome |
| `apps/desktop/src/settings/SettingsShell.module.css` | shell styles (ported mockup inline styles) |
| `apps/desktop/src/settings/SettingsShell.test.tsx` | §6 chrome/nav tests |
| `apps/desktop/src/settings/DetailHeader.tsx` | shared header component |
| `apps/desktop/src/settings/DetailHeader.module.css` | header styles |
| `apps/desktop/src/settings/DetailHeader.test.tsx` | §6 header tests |
| `apps/desktop/src/settings/pages.ts` | `Detail`, `SettingsPageName`, `PageComponentProps`, `PAGE_REGISTRY`, `NAV_GROUPS`, `PAGE_META` |
| `apps/desktop/src/main-settings.tsx` | `settings.html` entry: ToastProvider + QueryClientProvider + host resolution + `<SettingsApp/>` |

U08–U14 own their page directories (`apps/desktop/src/settings/pages/<Page>/…`) and add ONLY their one import line to `PAGE_REGISTRY` in `pages.ts` (the table below reserves the keys; the lazy import paths are fixed here so edits cannot collide).

## 2. Types (`pages.ts`)

```ts
import type { EngineHost } from '@which-model/core'

/** Which detail view is open over the current page's list. */
export type Detail =
  | { kind: 'profile'; id: string }   // Profiles page; id = profile slug
  | { kind: 'group'; id: string }     // Benchmark groups; id = group slug
  | { kind: 'benchmark'; id: string;  // Benchmark groups; id = benchmark name
      fromGroup: string | null }      // originating group slug; back returns there
  | { kind: 'provider'; id: string }  // Providers; id = provider id
  | { kind: 'provider-model'; provider: string; modelName: string; reasoning: string }
  | { kind: 'model'; id: string }     // Models page; id = catalog display name
  | { kind: 'harness'; id: string }   // Harnesses; id = harness slug

/** Props every registered page component receives — the ONLY interface U08–U14 implement to. */
export interface PageComponentProps {
  detail: Detail | null
  openDetail(d: Detail): void   // replaces detail wholesale
  closeDetail(): void           // SPEC §2.2 back semantics
}

export type SettingsPageName =
  | 'General' | 'Usage detection'
  | 'Profiles' | 'Benchmark groups' | 'Models' | 'Favourites'
  | 'Providers' | 'Harnesses' | 'Agent integration'

export type SettingsPageComponent = React.ComponentType<PageComponentProps>
```

Component props:

```ts
export interface SettingsAppProps { host: EngineHost }
export function SettingsApp(props: SettingsAppProps): JSX.Element

export interface SettingsShellProps {
  page: SettingsPageName
  onPage(page: SettingsPageName): void   // SettingsApp clears detail here
  configPath: string                      // '' while unknown
  onClose(): void                         // wired to host.window.closeSettings()
  children: React.ReactNode               // the mounted page component
}
export function SettingsShell(props: SettingsShellProps): JSX.Element

export interface DetailHeaderProps {
  title: string
  blurb: string
  backLabel?: string            // rendered with chevron when set
  onBack?(): void               // required when backLabel is set
  action?: { label: string; onAction(): void }
}
export function DetailHeader(props: DetailHeaderProps): JSX.Element
```

Host access for pages: `SettingsApp` provides `props.host` via context; hook `useHost(): EngineHost` is exported from `SettingsApp.tsx` and throws outside the provider.

## 3. Nav structure constant

```ts
export const NAV_GROUPS: ReadonlyArray<readonly [string, ReadonlyArray<SettingsPageName>]> = [
  ['app',     ['General', 'Usage detection']],
  ['ranking', ['Profiles', 'Benchmark groups', 'Models', 'Favourites']],
  ['routing', ['Providers', 'Harnesses', 'Agent integration']],
] as const
```

## 4. Page registry

```ts
/** name → lazy page component; one line per page, import path fixed here. */
export const PAGE_REGISTRY: Record<SettingsPageName, React.LazyExoticComponent<SettingsPageComponent>> = {
  'General':           React.lazy(() => import('./pages/General/GeneralPage')),            // U12
  'Usage detection':   React.lazy(() => import('./pages/Usage/UsagePage')),                // U13
  'Profiles':          React.lazy(() => import('./pages/Profiles/ProfilesPage')),          // U08
  'Benchmark groups':  React.lazy(() => import('./pages/Groups/GroupsPage')),              // U09
  'Favourites':        React.lazy(() => import('./pages/Favourites/FavouritesPage')),      // U14
  'Models':            React.lazy(() => import('./pages/Models/ModelsPage')),              // U15
  'Providers':         React.lazy(() => import('./pages/Providers/ProvidersPage')),        // U10
  'Harnesses':         React.lazy(() => import('./pages/Harnesses/HarnessesPage')),        // U11
  'Agent integration': React.lazy(() => import('./pages/Agent/AgentPage')),                // U14
}
```

Each module's default export must satisfy `SettingsPageComponent`.

## 5. PAGE_META (verbatim from mockup lines 947–956; `[title, blurb, actionLabel | null]`)

```ts
export const PAGE_META: Record<SettingsPageName, readonly [string, string, string | null]> = {
  'Profiles': ['Profiles', 'Built-in profiles are read-only; duplicate one to edit its weights.', 'New profile'],
  'Benchmark groups': ['Benchmark groups', 'A group bundles benchmarks into one signal a profile can weight. Built-in groups are read-only — duplicate one to change what it contains.', 'New group'],
  'Harnesses': ['Harnesses', 'Detected automatically. {model_id} and {reasoning} are filled from the pick. Open one to choose which providers it may use.', 'Add custom'],
  'Favourites': ['Favourites', 'Pinned models are offered first when they rank in range for the profile.', 'Add model'],
  'Models': ['Models', 'Every model in the catalog. Open one for its identity, reasoning levels, and catalog scores.', null],
  'General': ['General', 'How which-model runs on this Mac, and how the pick is drawn in the popover.', null],
  'Usage detection': ['Usage detection', 'Where limits are read from, and how often.', null],
  'Providers': ['Providers', 'Drag to set priority — highest at the top. Default-deny: a provider is never read until you enable it. Open one to choose which of its models may be routed to.', null],
  'Agent integration': ['Agent integration', 'How coding agents reach which-model without the popover.', null],
}
```

Pages render `title`/`blurb` in list state; `actionLabel` non-null ⇒ pass `action` to `DetailHeader` with that label (the handler is the page's own).

## 6. Test fixtures (write first — TDD)

All tests: vitest + `@testing-library/react`, host = `createMockEngineHost()` from `@which-model/core/mock`; registry entries stubbed with synchronous fakes exposing `openDetail`/`closeDetail` buttons where noted.

| Test | Asserts |
|---|---|
| `SettingsApp.test` nav clears detail | open `{kind:'profile', id:'research'}` via stub page → click sidebar `General` → stub receives `detail: null`; re-clicking the CURRENT page also clears detail |
| `SettingsApp.test` back to list | `openDetail({kind:'provider', id:'claude'})` → `closeDetail()` → `detail` is `null` (same for profile/group/harness kinds) |
| `SettingsApp.test` benchmark→group back | `openDetail({kind:'group', id:'agentic_coding'})` → `openDetail({kind:'benchmark', id:'SWE-Bench Verified', fromGroup:'agentic_coding'})` → `closeDetail()` → `detail` = `{kind:'group', id:'agentic_coding'}` → `closeDetail()` → `null`; with `fromGroup: null` the first `closeDetail()` goes straight to `null` |
| `SettingsShell.test` close calls host | click the close dot → `host.window.closeSettings` called exactly once; titlebar has `--wails-draggable: drag`, close dot `--wails-draggable: no-drag` |
| `SettingsShell.test` chrome | title text `which-model — Settings`; three dots rendered, only the first clickable; all 8 `NAV_GROUPS` items under labels `app`/`ranking`/`routing`; active item (and only it) shows the accent ring; footer shows `config_path` from the mock's `GUISettings` |
| `DetailHeader.test` PAGE_META rendering | for each of the 8 pages: render with `PAGE_META` values → exact title + blurb text; action button rendered iff `actionLabel !== null` and fires `onAction`; back link absent without `backLabel`, present with it, fires `onBack` |
| `main-settings` smoke (in `SettingsApp.test`) | rendering the root tree provides ToastProvider + QueryClient (a stub page calling `useToast` and `useHost` does not throw) |

Verify: `pnpm --filter desktop test -- settings` (plus `pnpm -r typecheck`).


## Correction (2026-09-05)

The Profiles / Use Cases correction in `specs/desktop/backend/features/B03-profiles/SPEC.md` governs the new persisted profile selection and desktop terminology. The DTO extension is canonical in `specs/desktop/global/CONTRACTS.md`. Settings navigation now has both Profiles (curated defaults) and Use Cases (ranking presets).
