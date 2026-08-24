import type { EngineHost } from './host.js';
import type { GUISettings, HarnessInfo, ProfileDetail } from './types.js';
export declare const MOCK_NOW = "2026-01-01T12:00:00Z";
export interface MockData {
    profiles: ProfileDetail[];
    models: MockModel[];
    groups: {
        slug: string;
        builtin: boolean;
        benchmarks: string[];
    }[];
    benchmarks: string[];
    harnesses: HarnessInfo[];
    providers: MockProvider[];
    favourites: string[];
    routesDisabled: string[];
    settings: GUISettings;
}
export interface MockModel {
    id: string;
    name: string;
    reasoning: string;
    providers: string[];
    core: {
        intelligence: number;
        cost: number;
        speed: number;
    };
    groupScores: Record<string, number>;
}
export interface MockProvider {
    id: string;
    on: boolean;
    priority: number;
    auth: string;
    limits: string;
    session: number | null;
    weekly: number | null;
    monthly: number | null;
    credits: string;
    resets: string;
}
export declare function createMockEngineHost(overrides?: Partial<MockData>): EngineHost & {
    data: MockData;
};
//# sourceMappingURL=mock.d.ts.map