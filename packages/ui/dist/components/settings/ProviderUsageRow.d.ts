import type { ReactNode } from 'react';
import type { ProviderInfo } from '@which-model/core';
export interface ProviderUsageRowProps {
    provider: ProviderInfo;
    /** The switch's state. Its MEANING is the caller's: global enablement on the
     *  Providers page, per-harness permission on a harness detail. */
    on: boolean;
    onToggle(next: boolean): void;
    /** When false the meters, credits and resets read blank and the subtitle
     *  shows `offLabel` — a provider that cannot report has no usage, and
     *  showing a stale bar would misrepresent remaining quota. */
    live: boolean;
    /** Subtitle when `live` is false. Harness detail says "off globally"
     *  (the provider is on, but disabled app-wide); the Providers page says
     *  "not enabled". */
    offLabel?: string;
    /** Extra content after the credits column (e.g. a route count, a chevron). */
    trailing?: ReactNode;
    /** Content between the switch and the id (e.g. a priority number). Kept
     *  inside the card so it does not consume the flexible meter column. */
    leading?: ReactNode;
}
/**
 * One provider as a usage card: switch, id over its auth line, the three usage
 * meters, then credits over the reset hint.
 *
 * Ported from the harness detail view (mockup demo.dc.html 542-566, bindings
 * 1096-1118) and shared, because the same row is the Providers page's list —
 * it carries the live quota picture that a plain id-and-count row cannot.
 */
export declare function ProviderUsageRow({ provider, on, onToggle, live, offLabel, trailing, leading, }: ProviderUsageRowProps): import("react").JSX.Element;
