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
  | 'Usage detection'
  | 'Profiles'
  | 'Benchmark groups'
  | 'Favourites'
  | 'Providers'
  | 'Harnesses'
  | 'Agent integration'

export type SettingsPageComponent = React.ComponentType<PageComponentProps>

export interface SettingsAppProps {
  host: EngineHost
}

export interface SettingsShellProps {
  page: SettingsPageName
  onPage(page: SettingsPageName): void // SettingsApp clears detail here
  configPath: string // '' while unknown
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
  ['app', ['General', 'Usage detection']],
  ['ranking', ['Profiles', 'Benchmark groups', 'Favourites']],
  ['routing', ['Providers', 'Harnesses', 'Agent integration']],
] as const

/** name → lazy page component; one line per page, import path fixed here. */
export const PAGE_REGISTRY: Record<
  SettingsPageName,
  React.LazyExoticComponent<SettingsPageComponent>
> = {
  General: React.lazy(() => import('./pages/General/GeneralPage')),
  'Usage detection': React.lazy(() => import('./pages/Usage/UsagePage')),
  Profiles: React.lazy(() => import('./pages/Profiles/ProfilesPage')),
  'Benchmark groups': React.lazy(() => import('./pages/Groups/GroupsPage')),
  Favourites: React.lazy(() => import('./pages/Favourites/FavouritesPage')),
  Providers: React.lazy(() => import('./pages/Providers/ProvidersPage')),
  Harnesses: React.lazy(() => import('./pages/Harnesses/HarnessesPage')),
  'Agent integration': React.lazy(() => import('./pages/Agent/AgentPage')),
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
  General: [
    'General',
    'How which-model runs on this Mac, and how the pick is drawn in the popover.',
    null,
  ],
  'Usage detection': [
    'Usage detection',
    'Where limits are read from, and how often.',
    null,
  ],
  Providers: [
    'Providers',
    'Drag to set priority — highest at the top. Default-deny: a provider is never read until you enable it. Open one to choose which of its models may be routed to.',
    null,
  ],
  'Agent integration': [
    'Agent integration',
    'How coding agents reach which-model without the popover.',
    null,
  ],
}