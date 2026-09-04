---
kind: feature-contracts
version: "1.0"
feature: U00-ui
project: which-model-desktop
---

# U00-ui — Contracts

## 1. Packages

| Package | name | deps | build |
|---|---|---|---|
| `packages/core` | `@which-model/core` | none (dev: typescript, vitest) | `tsc -p tsconfig.json` → `dist/` (ESM + d.ts) |
| `packages/ui` | `@which-model/ui` | `@dnd-kit/core`, `@dnd-kit/sortable`; peer `react`, `react-dom` (>=18) | `tsc` + CSS copied as-is (`exports` maps `./styles.css` bundle and per-module CSS) |

Both: `"type": "module"`, TS `strict: true`, target ES2022, JSX `react-jsx`. Apps consume via workspace protocol (`"@which-model/core": "workspace:*"`).

## 2. Component conventions

```
packages/ui/src/components/<Name>/
  <Name>.tsx          // export function <Name>(props: <Name>Props)
  <Name>.module.css
  <Name>.test.tsx
```
- Props interfaces are exported and named `<Name>Props`; callbacks named `on<Event>`; boolean flags positive (`disabled`, not `enabled:false` patterns).
- Numeric prop ranges follow D00 CONTRACTS §6 tokens (weights 0–5, balance 10..90 step 5, complexity stop 0..4).
- Every component renders correctly inside a plain `<div>` — no required context except `ToastProvider` for `useToast`.
- Class merging: local `cx(...classNames)` util in `packages/ui/src/lib/cx.ts` (owned by U02); no external classnames dep.

## 3. Shared hook (owned by U02, used by U03)

```ts
// usePointerFraction: replicate mockup drag() semantics (U00 SPEC §2.4).
export function usePointerFraction(
  onFraction: (f: number) => void,
): (e: React.PointerEvent<HTMLElement>) => void  // attach as onPointerDown
```

## 4. Mock fixtures (owned by U01; UI tests import from `@which-model/core/mock`)

`createMockEngineHost(overrides?: Partial<MockData>): EngineHost & { data: MockData }`
`MockData` must contain: 8 models × providers/reasoning as in the mockup's `MODELS`; the 5 complexity-scale profile slugs matching D00 order; 11 builtin group slugs from the mockup's `GROUP_DEFS`; the full `ALL_BENCH` benchmark list; 4 harnesses (claude/codex/copilot/cursor seeds); providers claude/codex/copilot enabled + cursor disabled with the mockup's usage numbers; default `GUISettings`. All mutations update `data` and fire the appropriate event on registered listeners — UI tests assert refetch behaviour through it.

## 5. Event→query invalidation map (apps; owned by U05's `invalidate.ts`)

| Event | Invalidated query keys |
|---|---|
| `config:changed` | `['profiles']`, `['profile', *]`, `['providers']`, `['provider', *]`, `['harnesses']`, `['favourites']`, `['rank']*`, `['catalog-line']`, `['catalog-models']` |
| `catalog:changed` | `['groups']`, `['group', *]`, `['benchmarks']`, `['benchmark', *]`, `['catalog-models']`, `['rank']*` |
| `usage:updated` | `['usage']`, `['providers']` |
| `settings:changed` | `['settings']`, `['rank']*` |
| `pick:recorded` | `['profiles']`, `['profile', *]` (pick counts), `['catalog-line']` |

`['rank']*` = prefix invalidation of all rank queries.

## 6. Query keys (canonical; features cite)

`['profiles']` `['profile', slug]` `['complexity-scale']` `['rank', slug, overridesHash, holds]` `['catalog-line']` `['groups']` `['group', slug]` `['benchmarks']` `['benchmark', name]` `['catalog-models']` `['providers']` `['provider', id]` `['harnesses']` `['usage', force]` `['favourites']` `['settings']` `['snippets']`
`overridesHash` = stable JSON stringify of the overrides DTO or `'none'`.

## 7. Visual tokens

See D00 CONTRACTS §6 (single source). UI feature contracts reference tokens by name (e.g. "toggle geometry per D00 §6") and only state geometry NOT already listed there.
