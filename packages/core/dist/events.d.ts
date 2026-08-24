import type { GUISettings } from './types.js';
export declare const ENGINE_EVENTS: readonly ['config:changed', 'catalog:changed', 'usage:updated', 'settings:changed', 'pick:recorded'];
export type EngineEvent = (typeof ENGINE_EVENTS)[number];
export type EngineEventPayloads = {
    'config:changed': {
        section: string;
    };
    'catalog:changed': Record<string, never>;
    'usage:updated': Record<string, never>;
    'settings:changed': GUISettings;
    'pick:recorded': {
        profile_slug: string;
        route_key: string;
    };
};
//# sourceMappingURL=events.d.ts.map