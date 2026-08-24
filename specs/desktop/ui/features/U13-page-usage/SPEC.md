---
kind: feature-spec
version: "1.0"
feature: U13-page-usage
project: which-model-desktop
---

# U13-page-usage — Usage Detection Page

## 1. Purpose

`UsagePage` (`apps/desktop/src/settings/pages/usage/`) is the "Usage detection" page inside the U07 settings shell: two segmented controls choosing how usage is read (mode, backend), and a "live limits" list rendering one row per provider `UsageDTO`.

Depends on: U02, U07. Inherits D00 + U00. Mockup `onUsage` block of `specs/desktop/mockup/demo.dc.html` is normative.

## 2. Behaviour

1. **"detection" section.** Uppercase mono label `detection`. Row 1: title "Read usage from", note "Where remaining quota is measured before a pick.", right-aligned mono `SegmentedControl` with options `auto` / `on` / `off`; selecting writes `host.usage.setMode(v)`. Row 2: label "Backend", seg `off` / `native` / `codexbar`; selecting writes `host.usage.setBackend(v)`. Current values come from query `['usage-mode']` → `host.usage.mode()`; both mutations invalidate `['usage-mode']` and `['usage']` on success. Clicking the selected option fires nothing.

2. **"live limits" section.** Uppercase label `live limits`, then one row per element of query `['usage', false]` → `host.usage.snapshots(false)`, in returned order. The page refetches when the `usage:updated` event fires (already covered by U00 CONTRACTS §5: `usage:updated` → `['usage']`).

3. **Row layout** (mockup `usageRows`): mono provider id, 82px fixed; centre column (flex) stacking the limits text (mono 10.5px, 55% text colour) over a 4px bar (radius 3, 10%-text track, `--color-accent-500` fill); right column `UsageDTO.Auth` (mono 10.5px, 80px fixed, right-aligned, 42% text colour; `—` when `auth` is empty).

4. **Bar percent.** Fill width = `max(w.used_percent)` over `windows` entries where `used_percent` is non-null and `unlimited` is false, clamped to 0..100. When no window qualifies, width is `0%`.

5. **Limits text composition.** Join, in `windows` order and separated by `" · "`, one segment per window: `"{id} {used_percent}%"` for windows with non-null `used_percent`; `"{id} unlimited"` for `unlimited` windows; windows with null `used_percent` (and not unlimited) are skipped. Example: `session 62% · weekly 41% · monthly 18%`.

6. **Fallbacks** (in precedence order, per row):
   a. `failure` non-empty → the centre column shows the failure text alone, muted (42% text colour), and the bar renders empty (0%).
   b. Otherwise, no windows at all or every window skipped by §2.5 → limits text is `no usage data`, muted, bar empty.

7. **Page chrome** (U07 registry): title "Usage detection", blurb "Where limits are read from, and how often.", no page action.

## 3. Error behaviour

- Rejected `setMode`/`setBackend` toasts `ErrorDTO.message`; segs re-render from the query cache (no optimistic state).
- `['usage', false]` rejecting with `usage_unavailable` (or any error) → the live-limits section renders an inline error state with retry (U00 SPEC §3); the detection segs stay usable.
- Empty snapshot list → `EmptyState` (U02) with text `no providers enabled`.

## 4. Decisions

| Decision | Value | Rationale |
|---|---|---|
| Mode query key | Feature-owned `['usage-mode']`, invalidated in the mutations' `onSuccess` | `usage.mode()` has no canonical key in U00 §6; `config:changed`'s map doesn't cover it, so the page owns its freshness |
| Bar reduction | Max non-null, non-unlimited `used_percent` across windows | The binding constraint is what stops you (same rationale as F19 SPEC §2.1); a mean would hide an exhausted lane |
| Limits text | `id`-keyed segments joined by `" · "`; unknown windows skipped, unlimited spelled out | Mockup shows precomposed strings; this derivation reproduces them from `UsageDTO` deterministically |
| Failure display | Failure text replaces limits+bar, muted | `UsageDTO.Failure` is already sanitised (D00 §2); a row with both would misread as live data |
| snapshots force flag | Always `false` on this page | Background refresher owns forcing; the page just mirrors state on `usage:updated` |
