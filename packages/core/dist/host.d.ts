import type { BenchmarkDetail, CatalogSummary, Favourite, GroupDetail, GroupSummary, GUISettings, HarnessInfo, LaunchResult, CatalogModel, CatalogModelDetail, ModelScoreDetail, ProfileDetail, ProfileSummary, ProviderAccount, ProviderDetail, ProviderInfo, RankRequest, RankResponse, ShellSnippets, UsageDTO } from './types.js';
import type { EngineEvent } from './events.js';
export interface EngineHost {
    profiles: {
        list(): Promise<ProfileSummary[]>;
        get(slug: string): Promise<ProfileDetail>;
        save(p: ProfileDetail): Promise<void>;
        duplicate(slug: string): Promise<ProfileDetail>;
        delete(slug: string): Promise<void>;
        complexityScale(): Promise<string[]>;
    };
    pick: {
        rank(req: RankRequest): Promise<RankResponse>;
        recordPick(profileSlug: string, routeKey: string): Promise<void>;
        catalogLine(): Promise<CatalogSummary>;
    };
    catalog: {
        benchmarks(): Promise<string[]>;
        benchmarkDetail(name: string): Promise<BenchmarkDetail>;
        /** Benchmarks for one (model, reasoning) pair. Empty rows if untested. */
        modelDetail(model: string, reasoning: string): Promise<ModelScoreDetail>;
        /** Distinct catalog models (scores CSV), name ascending. */
        models(): Promise<CatalogModel[]>;
        /** Model card: identity plus enabled providers and per-provider prices. */
        model(name: string): Promise<CatalogModelDetail>;
        groups(): Promise<GroupSummary[]>;
        groupDetail(slug: string): Promise<GroupDetail>;
        saveGroup(slug: string, benchmarks: string[], renameTo?: string): Promise<void>;
        duplicateGroup(slug: string): Promise<GroupDetail>;
        deleteGroup(slug: string): Promise<void>;
    };
    providers: {
        /** Register a provider id (default-deny). Ids come from `addable()`. */
        add(id: string): Promise<void>;
        /** models.dev provider slugs not already listed — the only ids that can
         *  acquire routes, so the add control offers these rather than free text.
         *  Empty when the catalogue cache has not been built yet. */
        addable(): Promise<string[]>;
        /** Remove a provider's config entry and its routes. Builtins reject. */
        delete(id: string): Promise<void>;
        /** Copy a provider (accounts included) to a fresh id, which is returned. */
        duplicate(id: string): Promise<string>;
        /** Replace a provider's account list wholesale — one atomic write covers
         *  add, rename, re-kind and remove. */
        setAccounts(id: string, accounts: ProviderAccount[]): Promise<void>;
        list(): Promise<ProviderInfo[]>;
        setEnabled(id: string, on: boolean): Promise<void>;
        reorder(orderedIds: string[]): Promise<void>;
        detail(id: string): Promise<ProviderDetail>;
        setRouteEnabled(id: string, modelId: string, reasoning: string, on: boolean): Promise<void>;
        setAllRoutes(id: string, on: boolean): Promise<void>;
        /** Rebuild the route table from the model catalogue (same as
         *  `which-model routes refresh`). Call after sign-in and from Refresh models. */
        refreshRoutes(): Promise<void>;
    };
    harnesses: {
        list(): Promise<HarnessInfo[]>;
        save(h: HarnessInfo): Promise<void>;
        delete(slug: string): Promise<void>;
        setEnabled(slug: string, enabled: boolean): Promise<void>;
        setProvider(slug: string, provider: string, on: boolean): Promise<void>;
        setAllProviders(slug: string, on: boolean): Promise<void>;
        launch(slug: string, routeKey: string, profileSlug: string): Promise<LaunchResult>;
    };
    usage: {
        snapshots(force: boolean): Promise<UsageDTO[]>;
        setMode(mode: 'auto' | 'on' | 'off'): Promise<void>;
        setBackend(backend: 'off' | 'native' | 'codexbar'): Promise<void>;
        mode(): Promise<{
            mode: string;
            backend: string;
        }>;
    };
    favourites: {
        list(): Promise<Favourite[]>;
        pin(routeKey: string): Promise<void>;
        unpin(routeKey: string): Promise<void>;
    };
    settings: {
        get(): Promise<GUISettings>;
        set(s: GUISettings): Promise<void>;
        shellSnippets(): Promise<ShellSnippets>;
    };
    signin: {
        /** Begin OAuth and return the unguessable id required by follow-up calls. */
        start(provider: string): Promise<{
            flow_id: string;
            verification_uri: string;
            user_code: string;
            paste_required: boolean;
        }>;
        /** Poll / wait until approved/expired/denied, save the credential, and
         *  associate it with the named account. Long-running: call off render. */
        confirm(provider: string, flowId: string, accountName: string): Promise<void>;
        /** Deliver a pasted Claude authentication code to an in-flight confirm. */
        submitCode(provider: string, flowId: string, code: string): Promise<void>;
        /** Store an API key securely and add its non-secret account reference. */
        saveAPIKey(provider: string, accountName: string, apiKey: string): Promise<void>;
        /** Abandon the exact active flow; stale ids cannot cancel replacements. */
        cancel(provider: string, flowId: string): Promise<void>;
    };
    window: {
        openSettings(): Promise<void>;
        closeSettings(): Promise<void>;
        hidePopover(): Promise<void>;
        quit(): Promise<void>;
        copyToClipboard(text: string): Promise<void>;
        /** Open a URL in the user's default browser (device-flow verification). */
        openURL(url: string): Promise<void>;
        /** Resize the popover window to its content's natural height (px,
         *  clamped host-side to [320, 620]) so the panel is content-sized like
         *  the design instead of a fixed 620 with filler. */
        setPopoverHeight(height: number): Promise<void>;
        /** Put the popover's current pick in the menu bar. The host ranks for the
         *  menu bar itself at startup, but cannot see the active profile or the
         *  ephemeral weight overrides — both live here — so the popover pushes
         *  what it shows and owns the title from the first push. */
        setTrayPick(profileName: string, modelName: string, reasoning: string, provider: string): Promise<void>;
    };
    on(event: EngineEvent, cb: (payload: unknown) => void): () => void;
}
//# sourceMappingURL=host.d.ts.map