import type {
  BenchmarkDetail,
  CatalogSummary,
  Favourite,
  GroupDetail,
  GroupSummary,
  GUISettings,
  HarnessInfo,
  LaunchResult,
  ModelScoreDetail,
  ProfileDetail,
  ProfileSummary,
  ProviderAccount,
  ProviderDetail,
  ProviderInfo,
  RankRequest,
  RankResponse,
  ShellSnippets,
  UsageDTO,
} from './types.js'
import type { EngineEvent } from './events.js'

// EngineHost — verbatim from D00 CONTRACTS §5. Shape changes only via D00.
export interface EngineHost {
  profiles: {
    list(): Promise<ProfileSummary[]>
    get(slug: string): Promise<ProfileDetail>
    save(p: ProfileDetail): Promise<void>
    duplicate(slug: string): Promise<ProfileDetail>
    delete(slug: string): Promise<void>
    complexityScale(): Promise<string[]>
  }
  pick: {
    rank(req: RankRequest): Promise<RankResponse>
    recordPick(profileSlug: string, routeKey: string): Promise<void>
    catalogLine(): Promise<CatalogSummary>
  }
  catalog: {
    benchmarks(): Promise<string[]>
    benchmarkDetail(name: string): Promise<BenchmarkDetail>
    /** Benchmarks for one (model, reasoning) pair. Empty rows if untested. */
    modelDetail(model: string, reasoning: string): Promise<ModelScoreDetail>
    groups(): Promise<GroupSummary[]>
    groupDetail(slug: string): Promise<GroupDetail>
    saveGroup(slug: string, benchmarks: string[], renameTo?: string): Promise<void>
    duplicateGroup(slug: string): Promise<GroupDetail>
    deleteGroup(slug: string): Promise<void>
  }
  providers: {
    /** Register a provider id (default-deny). Ids come from `addable()`. */
    add(id: string): Promise<void>
    /** models.dev provider slugs not already listed — the only ids that can
     *  acquire routes, so the add control offers these rather than free text.
     *  Empty when the catalogue cache has not been built yet. */
    addable(): Promise<string[]>
    /** Remove a provider's config entry and its routes. Builtins reject. */
    delete(id: string): Promise<void>
    /** Copy a provider (accounts included) to a fresh id, which is returned. */
    duplicate(id: string): Promise<string>
    /** Replace a provider's account list wholesale — one atomic write covers
     *  add, rename, re-kind and remove. */
    setAccounts(id: string, accounts: ProviderAccount[]): Promise<void>
    list(): Promise<ProviderInfo[]>
    setEnabled(id: string, on: boolean): Promise<void>
    reorder(orderedIds: string[]): Promise<void>
    detail(id: string): Promise<ProviderDetail>
    setRouteEnabled(id: string, modelId: string, reasoning: string, on: boolean): Promise<void>
    setAllRoutes(id: string, on: boolean): Promise<void>
    /** Rebuild the route table from the model catalogue (same as
     *  `which-model routes refresh`). Call after sign-in and from Refresh models. */
    refreshRoutes(): Promise<void>
  }
  harnesses: {
    list(): Promise<HarnessInfo[]>
    save(h: HarnessInfo): Promise<void>
    delete(slug: string): Promise<void>
    setProvider(slug: string, provider: string, on: boolean): Promise<void>
    setAllProviders(slug: string, on: boolean): Promise<void>
    launch(slug: string, routeKey: string, profileSlug: string): Promise<LaunchResult>
  }
  usage: {
    snapshots(force: boolean): Promise<UsageDTO[]>
    setMode(mode: 'auto' | 'on' | 'off'): Promise<void>
    setBackend(backend: 'off' | 'native' | 'codexbar'): Promise<void>
    mode(): Promise<{ mode: string; backend: string }>
  }
  favourites: {
    list(): Promise<Favourite[]>
    pin(routeKey: string): Promise<void>
    unpin(routeKey: string): Promise<void>
  }
  settings: {
    get(): Promise<GUISettings>
    set(s: GUISettings): Promise<void>
    shellSnippets(): Promise<ShellSnippets>
  }
  signin: {
    /** Begin OAuth: returns the URL to open and, for device-code providers,
     *  the user code. Claude returns an empty user_code (paste the callback). */
    start(provider: string): Promise<{ verification_uri: string; user_code: string }>
    /** Poll / wait until approved/expired/denied and save the credential.
     *  Long-running: call off the render path. Rejects with ErrorDTO. */
    confirm(provider: string): Promise<void>
    /** Deliver a pasted Claude authentication code to an in-flight confirm. */
    submitCode(provider: string, code: string): Promise<void>
    /** Abandon an active flow (safe to call anytime). */
    cancel(provider: string): Promise<void>
  }
  window: {
    openSettings(): Promise<void>
    closeSettings(): Promise<void>
    hidePopover(): Promise<void>
    quit(): Promise<void>
    copyToClipboard(text: string): Promise<void>
    /** Open a URL in the user's default browser (device-flow verification). */
    openURL(url: string): Promise<void>
    /** Resize the popover window to its content's natural height (px,
     *  clamped host-side to [320, 620]) so the panel is content-sized like
     *  the design instead of a fixed 620 with filler. */
    setPopoverHeight(height: number): Promise<void>
    /** Put the popover's current pick in the menu bar. The host ranks for the
     *  menu bar itself at startup, but cannot see the active profile or the
     *  ephemeral weight overrides — both live here — so the popover pushes
     *  what it shows and owns the title from the first push. */
    setTrayPick(
      profileName: string,
      modelName: string,
      reasoning: string,
      provider: string,
    ): Promise<void>
  }
  on(event: EngineEvent, cb: (payload: unknown) => void): () => void
}
