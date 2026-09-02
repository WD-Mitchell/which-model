// Settings page registry contract (U07 CONTRACTS §2–§5). Owned by U07; page
// features (U08–U14) implement the <Page> modules this registry lazy-imports.
import type { EngineHost } from '@which-model/core'
import * as React from 'react'

/** Which detail view is open over the current page's list. */
export type Detail =
  | { kind: 'profile'; id: string } // Profiles page; id = profile slug
  | { kind: 'group'; id: string } // Benchmark groups; id = group slug
  | {
      kind: 'benchmark'
      id: string // Benchmark groups; id = benchmark name
      fromGroup: string | null // originating group slug; back returns there
    }
  | { kind: 'provider'; id: string } // Providers; id = provider id
  | {
      kind: 'provider-model'
      provider: string
      modelName: string
      reasoning: string
    } // Providers; one model+reasoning combo's benchmarks
  | { kind: 'model'; id: string; fromProvider?: string } // Models page (or Providers click-through); id = catalog display name
  | { kind: 'harness'; id: string } // Harnesses; id = harness slug

/** Props every registered page component receives — the ONLY interface
 * U08–U14 implement to. */
export interface PageComponentProps {
  detail: Detail | null
  openDetail(d: Detail): void // replaces detail wholesale
  closeDetail(): void // SPEC §2.2 back semantics
}

export type SettingsPageName =
  | 'General'
  | 'Profiles'
  | 'Benchmark groups'
  | 'Models'
  | 'Favourites'
  | 'Providers'
  | 'Harnesses'
export type SettingsPageComponent = React.ComponentType<PageComponentProps>

export interface SettingsAppProps {
  host: EngineHost
}

export interface SettingsShellProps {
  page: SettingsPageName
  onPage(page: SettingsPageName): void // SettingsApp clears detail here
  configPath: string // '' while unknown
  version: string // '' while unknown
  onClose(): void // wired to host.window.closeSettings()
  children: React.ReactNode // the mounted page component
}

export interface DetailHeaderProps {
  title: string
  blurb: string
  backLabel?: string // rendered with chevron when set
  onBack?(): void // required when backLabel is set
  action?: { label: string; onAction(): void }
}

/** Nav structure constant (U07 CONTRACTS §3). */
export const NAV_GROUPS: ReadonlyArray<
  readonly [string, ReadonlyArray<SettingsPageName>]
> = [
  // 'Usage detection' was removed: its only remaining control (the usage
  // backend) now sits at the top of Providers, next to the live limits it
  // governs, and the per-provider limits list it duplicated is the Providers
  // list itself.
  ['app', ['General']],
  ['ranking', ['Profiles', 'Benchmark groups', 'Models', 'Favourites']],
  ['routing', ['Providers', 'Harnesses']],
] as const

/** name → lazy page component; one line per page, import path fixed here. */
export const PAGE_REGISTRY: Record<
  SettingsPageName,
  React.LazyExoticComponent<SettingsPageComponent>
> = {
  General: React.lazy(() => import('./pages/General/GeneralPage')),
  Profiles: React.lazy(() => import('./pages/Profiles/ProfilesPage')),
  'Benchmark groups': React.lazy(() => import('./pages/Groups/GroupsPage')),
  Favourites: React.lazy(() => import('./pages/Favourites/FavouritesPage')),
  Models: React.lazy(() => import('./pages/Models/ModelsPage')),
  Providers: React.lazy(() => import('./pages/Providers/ProvidersPage')),
  Harnesses: React.lazy(() => import('./pages/Harnesses/HarnessesPage')),
}

/** PAGE_META verbatim from U07 CONTRACTS §5 ([title, blurb, actionLabel|null]). */
export const PAGE_META: Record<
  SettingsPageName,
  readonly [string, string, string | null]
> = {
  Profiles: [
    'Profiles',
    'Built-in profiles are read-only; duplicate one to edit its weights.',
    'New profile',
  ],
  'Benchmark groups': [
    'Benchmark groups',
    'A group bundles benchmarks into one signal a profile can weight. Built-in groups are read-only — duplicate one to change what it contains.',
    'New group',
  ],
  Harnesses: [
    'Harnesses',
    'Detected automatically. {model_id} and {reasoning} are filled from the pick. Open one to choose which providers it may use.',
    'Add custom',
  ],
  Favourites: [
    'Favourites',
    'Pinned models are offered first when they rank in range for the profile.',
    'Add model',
  ],
  Models: [
    'Models',
    'Every model in the catalog. Open one for its identity, reasoning levels, and catalog scores.',
    null,
  ],
  General: [
    'General',
    'How which-model runs on this Mac, and how the pick is drawn in the popover.',
    null,
  ],
  Providers: [
    'Providers',
    'Drag to set priority — highest at the top. Default-deny: a provider is never read until you enable it. Open one to choose which of its models may be routed to.',
    // Deliberate addition over the mockup (user request): custom providers can
    // be registered here; they route nothing until routes are declared.
    'Add provider',
  ],
}