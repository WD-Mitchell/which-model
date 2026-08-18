---
kind: feature-contracts
version: "1.0"
feature: U13-page-usage
project: which-model-desktop
---

# U13-page-usage — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `apps/desktop/src/settings/pages/usage/UsagePage.tsx` | `UsagePage` + registry entry export |
| `apps/desktop/src/settings/pages/usage/limits.ts` | `usageBarPercent`, `usageLimitsText` (pure) |
| `apps/desktop/src/settings/pages/usage/UsagePage.module.css` | row/bar styles |
| `apps/desktop/src/settings/pages/usage/UsagePage.test.tsx` | fixtures §4 |
| `apps/desktop/src/settings/pages/usage/limits.test.ts` | fixtures §4 |

## 2. Exports

```ts
// UsagePage: no props; host via app context (U07); queries ['usage-mode'], ['usage', false].
export function UsagePage(): JSX.Element

// usageBarPercent: SPEC §2.4. null ⇒ render 0% width.
export function usageBarPercent(u: UsageDTO): number | null

// usageLimitsText: SPEC §2.5–2.6. Returns the centre-column text and whether it
// is a fallback (failure text or "no usage data") to be rendered muted.
export function usageLimitsText(u: UsageDTO): { text: string; muted: boolean }
```

## 3. Copy (verbatim)

| Slot | Text |
|---|---|
| Section labels | `detection`, `live limits` |
| Mode row | `Read usage from` / `Where remaining quota is measured before a pick.` |
| Mode seg options | `auto`, `on`, `off` |
| Backend row | `Backend`; seg options `off`, `native`, `codexbar` |
| No-windows fallback | `no usage data` |
| Empty list | `no providers enabled` |
| Missing auth | `—` |

## 4. Test fixtures (vitest; `createMockEngineHost`)

- **Seg writes.** Clicking `off` on the mode seg calls `usage.setMode('off')` exactly once; clicking `codexbar` calls `usage.setBackend('codexbar')` once; clicking the currently selected option calls nothing.
- **Bar math** (`limits.test.ts`): windows `[62, 41, 18]` → 62; `[41, null, 90]` → 90; unlimited window with `used_percent: 99` present is ignored (`[unlimited(99), 30]` → 30); all-null or empty → `null`; `120` clamps to `100`.
- **Limits text**: `[session 62, weekly 41, monthly 18]` → `session 62% · weekly 41% · monthly 18%`; unlimited window → `… unlimited` segment; null-percent window omitted; `failure: "token expired"` → `{ text: 'token expired', muted: true }` regardless of windows; no windows → `{ text: 'no usage data', muted: true }`.
- **Rows render** id at 82px column, auth right column (`—` when empty), bar width matching `usageBarPercent`.
- **Refetch on event.** Firing `usage:updated` on the mock host causes `usage.snapshots` to be called again and new percents to render.
