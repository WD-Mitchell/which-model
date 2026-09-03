import { create } from 'zustand'

// The models list unmounts while its detail view is on the stack (U07 renders
// the page's list OR its detail, never both), so list-local useState loses the
// controls on every detail round-trip (#142). This store owns the list
// controls at module level for the session: navigating into a detail and back
// re-mounts the list with exactly the controls it was left in. Pure state,
// like U06's overrides store — no host imports. Deliberately not persisted to
// storage; an app restart starts fresh.

/** First-open defaults for the catalog list. */
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
