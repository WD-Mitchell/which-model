import type { BenchmarkDetail, CatalogSummary, Favourite, GroupDetail, GroupSummary, GUISettings, HarnessInfo, LaunchResult, ProfileDetail, ProfileSummary, ProviderDetail, ProviderInfo, RankRequest, RankResponse, ShellSnippets, UsageDTO } from './types.js';
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
        groups(): Promise<GroupSummary[]>;
        groupDetail(slug: string): Promise<GroupDetail>;
        saveGroup(slug: string, benchmarks: string[], renameTo?: string): Promise<void>;
        duplicateGroup(slug: string): Promise<GroupDetail>;
        deleteGroup(slug: string): Promise<void>;
    };
    providers: {
        list(): Promise<ProviderInfo[]>;
        setEnabled(id: string, on: boolean): Promise<void>;
        reorder(orderedIds: string[]): Promise<void>;
        detail(id: string): Promise<ProviderDetail>;
        setRouteEnabled(id: string, modelId: string, reasoning: string, on: boolean): Promise<void>;
        setAllRoutes(id: string, on: boolean): Promise<void>;
    };
    harnesses: {
        list(): Promise<HarnessInfo[]>;
        save(h: HarnessInfo): Promise<void>;
        delete(slug: string): Promise<void>;
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
    window: {
        openSettings(): Promise<void>;
        closeSettings(): Promise<void>;
        hidePopover(): Promise<void>;
        quit(): Promise<void>;
        copyToClipboard(text: string): Promise<void>;
    };
    on(event: EngineEvent, cb: (payload: unknown) => void): () => void;
}
//# sourceMappingURL=host.d.ts.map