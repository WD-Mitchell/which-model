import { create } from 'zustand'

// The providers list unmounts while its detail view is on the stack (U07
// renders the page's list OR its detail, never both), so list-local useState
// loses the controls on every detail round-trip (#142). This store owns the
// list controls at module level for the session: navigating into a detail and
// back re-mounts the list with exactly the controls it was left in. Pure
// state, like U06's overrides store — no host imports. Deliberately not
// persisted to storage; an app restart starts fresh.

export type EnabledFilter = 'all' | 'enabled' | 'disabled'
export type ProviderSort =
  | 'name-asc'
  | 'name-desc'
  | 'models-desc'
  | 'models-asc'
  | 'enabled-first'
  | 'disabled-first'
  | 'priority'

/** First-open defaults; `sortMode` carries #140's enabled-first so the active
 * backends lead the page. */
export const PROVIDERS_LIST_INITIAL = {
  query: '',
  enabledFilter: 'all',
  sortMode: 'enabled-first',
} as const

interface ProvidersListState {
  query: string
  enabledFilter: EnabledFilter
  sortMode: ProviderSort
  setQuery(query: string): void
  setEnabledFilter(filter: EnabledFilter): void
  setSortMode(sort: ProviderSort): void
}

export const useProvidersListStore = create<ProvidersListState>((set) => ({
  ...PROVIDERS_LIST_INITIAL,
  setQuery: (query) => set({ query }),
  setEnabledFilter: (enabledFilter) => set({ enabledFilter }),
  setSortMode: (sortMode) => set({ sortMode }),
}))
