import { create } from 'zustand'

// Settings list views unmount while one of their detail views is on the stack
// (U07 renders the page's list OR its detail, never both), so list-local
// useState loses search/filter/sort on every detail round-trip (#142). This
// module owns that state at module level instead: navigating into a detail
// and back re-mounts the list with exactly the controls it was left in.
// Deliberately session-scoped — nothing here writes storage or config.

export type EnabledFilter = 'all' | 'enabled' | 'disabled'
export type ProviderSort =
  | 'name-asc'
  | 'name-desc'
  | 'models-desc'
  | 'models-asc'
  | 'enabled-first'
  | 'disabled-first'
  | 'priority'

/** First-open defaults for the Providers list; `sortMode` carries #140's
 * enabled-first so active backends lead the page. */
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

/** First-open defaults for the Models catalog list. */
export const MODELS_LIST_INITIAL = {
  query: '',
  selectedMakers: [] as string[],
  selectedProviders: [] as string[],
} as const

interface ModelsListState {
  query: string
  selectedMakers: string[]
  selectedProviders: string[]
  setQuery(query: string): void
  toggleMaker(maker: string): void
  toggleProvider(provider: string): void
  clearFilters(): void
}

export const useModelsListStore = create<ModelsListState>((set) => ({
  ...MODELS_LIST_INITIAL,
  setQuery: (query) => set({ query }),
  toggleMaker: (maker) =>
    set((s) => ({
      selectedMakers: s.selectedMakers.includes(maker)
        ? s.selectedMakers.filter((m) => m !== maker)
        : [...s.selectedMakers, maker],
    })),
  toggleProvider: (provider) =>
    set((s) => ({
      selectedProviders: s.selectedProviders.includes(provider)
        ? s.selectedProviders.filter((p) => p !== provider)
        : [...s.selectedProviders, provider],
    })),
  clearFilters: () => set({ query: '', selectedMakers: [], selectedProviders: [] }),
}))
