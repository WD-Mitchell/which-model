---
kind: feature-contracts
version: "1.0"
feature: U01-core-types-host
project: which-model-desktop
---

# U01-core-types-host — Contracts

## 1. Package and files

| File | Contents |
|---|---|
| `packages/core/package.json` | §2 exact fields |
| `packages/core/tsconfig.json` | strict, ES2022, `"module": "NodeNext"`, declaration + declarationMap, `outDir: "dist"`, `rootDir: "src"`, excludes `__tests__` |
| `packages/core/src/types.ts` | TS mirrors of every D00 CONTRACTS §2 DTO (SPEC §2.1) |
| `packages/core/src/events.ts` | `EngineEvent`, `ENGINE_EVENTS`, `EngineEventPayloads` (§5) |
| `packages/core/src/errors.ts` | `ErrorCode`, `EngineError`, `isEngineError` (§3) |
| `packages/core/src/host.ts` | `EngineHost` verbatim from D00 CONTRACTS §5 |
| `packages/core/src/mock.ts` | `MOCK_NOW`, `MockData`, `createMockEngineHost`, fixtures §4, scoring §7 |
| `packages/core/src/index.ts` | barrel: re-export types, events, errors, host — NOT mock |
| `packages/core/src/__tests__/types.test.ts` | compile-level DTO assertions (§8) |
| `packages/core/src/__tests__/mock.test.ts` | mock behaviour tests (§8) |

## 2. package.json (exact fields)

```json
{
  "name": "@which-model/core",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "main": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": { "types": "./dist/index.d.ts", "default": "./dist/index.js" },
    "./mock": { "types": "./dist/mock.d.ts", "default": "./dist/mock.js" }
  },
  "files": ["dist"],
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "typecheck": "tsc -p tsconfig.json --noEmit",
    "test": "vitest run"
  },
  "devDependencies": { "typescript": "…", "vitest": "…" }
}
```
No `dependencies` key. Version specifiers chosen at implementation time; the empty-runtime-deps invariant is contractual.

## 3. errors.ts / events.ts / mock.ts signatures

```ts
export type ErrorCode =
  | 'validation_failed' | 'builtin_readonly' | 'not_found' | 'conflict'
  | 'io_error' | 'usage_unavailable' | 'launch_failed'

export class EngineError extends Error implements ErrorDTO {
  readonly code: ErrorCode
  constructor(code: ErrorCode, message: string) // sets name = 'EngineError'
}
export function isEngineError(e: unknown): e is EngineError

// events.ts
export const ENGINE_EVENTS = ['config:changed', 'catalog:changed',
  'usage:updated', 'settings:changed', 'pick:recorded'] as const
export type EngineEvent = (typeof ENGINE_EVENTS)[number]
export type EngineEventPayloads = {
  'config:changed': { section: string }
  'catalog:changed': Record<string, never>
  'usage:updated': Record<string, never>
  'settings:changed': GUISettings
  'pick:recorded': { profile_slug: string; route_key: string }
}

// mock.ts
export const MOCK_NOW = '2026-01-01T12:00:00Z'
export interface MockData {
  profiles: ProfileDetail[]            // §4.2
  models: MockModel[]                  // §4.1 (fixture-only shape below)
  groups: { slug: string; builtin: boolean; benchmarks: string[] }[] // §4.3
  benchmarks: string[]                 // mockup ALL_BENCH, verbatim, same order
  harnesses: HarnessInfo[]             // §4.4
  providers: MockProvider[]            // §4.5
  favourites: string[]                 // route keys; seed: []
  routesDisabled: string[]             // route keys; seed: []
  settings: GUISettings                // §4.6
}
export interface MockModel {
  id: string; name: string; reasoning: string; providers: string[]
  core: { intelligence: number; cost: number; speed: number }
  groupScores: Record<string, number>  // builtin group slug → 0–5 (§4.1)
}
export interface MockProvider {
  id: string; on: boolean; priority: number; auth: string; limits: string
  session: number | null; weekly: number | null; monthly: number | null
  credits: string; resets: string
}
export function createMockEngineHost(
  overrides?: Partial<MockData>,
): EngineHost & { data: MockData }
```

## 4. Fixture data (seeded `MockData` — implement from these tables, do NOT re-read the mockup)

### 4.1 Models (8) — core scores and per-group scores (0–5)

| name | id | reasoning | providers |  int | cost | speed |
|---|---|---|---|---|---|---|
| GPT-5.6 Luna | `gpt-5.6-luna` | max | codex, copilot | 5.0 | 3.0 | 3.5 |
| Claude Opus 5 | `claude-opus-5` | max | claude | 4.9 | 2.6 | 3.2 |
| GPT-5.6 Sol | `gpt-5.6-sol` | high | copilot, codex | 4.4 | 4.0 | 4.4 |
| Claude Sonnet 5.2 | `claude-sonnet-5.2` | high | claude, copilot | 4.2 | 4.4 | 4.6 |
| Gemini 3.5 Ultra | `gemini-3.5-ultra` | max | cursor | 4.7 | 3.4 | 3.8 |
| Grok 5 Fast | `grok-5-fast` | medium | cursor, copilot | 3.8 | 4.7 | 5.0 |
| Qwen 3.5 Max | `qwen-3.5-max` | medium | cursor | 4.0 | 4.9 | 4.2 |
| Llama 5 405B | `llama-5-405b` | low | copilot | 3.5 | 5.0 | 4.0 |

`groupScores` (columns = builtin group slugs §4.3 in order: software_engineering, reasoning, knowledge, research, instruction_following, agentic_tools, evidence_capture, ui_visual, security, data_ml, finance):

| id | se | rea | kno | res | if | at | ec | ui | sec | dml | fin |
|---|---|---|---|---|---|---|---|---|---|---|---|
| gpt-5.6-luna | 4.9 | 4.6 | 4.6 | 4.8 | 4.4 | 4.7 | 4.4 | 4.2 | 4.0 | 4.5 | 4.3 |
| claude-opus-5 | 4.8 | 4.8 | 4.5 | 4.6 | 4.7 | 4.9 | 4.7 | 4.6 | 4.5 | 4.4 | 4.3 |
| gpt-5.6-sol | 4.3 | 4.0 | 4.2 | 4.1 | 4.2 | 4.0 | 4.0 | 3.8 | 3.9 | 4.0 | 3.9 |
| claude-sonnet-5.2 | 4.5 | 4.1 | 4.0 | 3.9 | 4.6 | 4.4 | 4.3 | 4.4 | 4.2 | 4.0 | 4.0 |
| gemini-3.5-ultra | 4.4 | 4.5 | 4.8 | 4.7 | 4.0 | 4.2 | 4.2 | 4.3 | 4.1 | 4.4 | 4.1 |
| grok-5-fast | 4.0 | 3.5 | 3.6 | 3.4 | 3.9 | 3.8 | 3.6 | 3.6 | 3.4 | 3.5 | 3.4 |
| qwen-3.5-max | 4.1 | 3.8 | 4.0 | 3.8 | 3.7 | 3.6 | 3.6 | 3.4 | 3.5 | 3.7 | 3.5 |
| llama-5-405b | 3.5 | 3.2 | 3.6 | 3.2 | 3.4 | 3.2 | 3.2 | 3.0 | 3.2 | 3.3 | 3.1 |

### 4.2 Profiles (8, all `builtin: true`; weights are `tier1_weights` (int/cost/speed) + `tier2_weights`)

| slug | name | core_share | tier1 | tier2 | picks | last_used |
|---|---|---|---|---|---|---|
| `simple_action_execution` | Simple Action | 75 | 2/5/5 | instruction_following 4, agentic_tools 3 | 312 | 2026-01-01T11:48:00Z |
| `simple_implementation` | Simple Implementation | 60 | 4/4/3 | software_engineering 4, instruction_following 3, agentic_tools 3 | 1284 | 2026-01-01T11:00:00Z |
| `balanced_implementation` | Balanced Implementation | 70 | 4/3/3 | software_engineering 5, agentic_tools 4, instruction_following 3 | 866 | 2025-12-31T12:00:00Z |
| `research` | Research | 60 | 4/4/2 | research 5, knowledge 4, agentic_tools 3 | 174 | 2025-12-29T12:00:00Z |
| `planning` | Planning | 60 | 5/2/2 | reasoning 5, research 4, knowledge 3 | 121 | "" |
| `review` | Review | 65 | 4/3/3 | instruction_following 5, security 3 | 58 | "" |
| `ui_ux` | UI / UX | 60 | 4/3/3 | ui_visual 5, software_engineering 4 | 43 | "" |
| `research_fast` | Research (fast) | 60 | 3/4/5 | research 5, knowledge 3 | 19 | "" |

`profiles.complexityScale()` resolves to the first five slugs above, in that order.

### 4.3 Groups (11, builtin) — slugs and benchmark lists exactly as the mockup's `GROUP_DEFS`: `software_engineering` (24 benchmarks, SWE-Bench Verified …), `reasoning` (5), `knowledge` (3), `research` (3), `instruction_following` (2), `agentic_tools` (4), `evidence_capture` (4), `ui_visual` (4), `security` (2), `data_ml` (4), `finance` (5). Copy the lists verbatim from `demo.dc.html` `GROUP_DEFS` (lines 814–826) into the fixture — the only permitted mockup read.

### 4.4 Harnesses (4; providers map = mockup `DETECTED`)

| slug | name | command | builtin | installed | providers on |
|---|---|---|---|---|---|
| `claude` | Claude Code | `claude --model {model_id} --reasoning {reasoning}` | true | true | claude, codex, copilot |
| `codex` | Codex CLI | `codex -m {model_id} -c reasoning={reasoning}` | true | true | codex, copilot |
| `copilot` | Copilot CLI | `copilot --model {model_id}` | true | true | copilot, cursor |
| `cursor` | Cursor | `cursor --model {model_id}` | true | false | cursor |

(Providers map contains all four ids; those not listed as "on" are `false`.)

### 4.5 Providers (priority = row order)

| id | on | auth | limits | session | weekly | monthly | credits | resets |
|---|---|---|---|---|---|---|---|---|
| claude | true | oauth | `session 42% · weekly 18%` | 42 | 18 | 54 | max 20× plan | session in 2h 40m |
| codex | true | oauth | `session 12% · weekly 31% · 340 credits` | 12 | 31 | 44 | 340 credits left | weekly on Mon |
| copilot | true | device flow | `monthly 1200 of 4800` | 8 | 25 | 25 | 1200 of 4800 premium | monthly on the 1st |
| cursor | false | via codexbar | `not enabled` | null | null | null | no plan detected | — |

### 4.6 Default GUISettings

`layout:"carousel"`, `weight_control:"slider"`, `holds:5`, `shortcut:"alt+space"`, `show_menu_bar_icon:true`, `launch_at_login:false`, `copy_command_instead:false`, `close_popover_after_launch:true`, `auto_update:true`, `auto_update_frequency:"daily"`, `mcp_server:false`, `claude_md_hint:false`, `shell_alias:false`, `use_keychain:true`, `config_path:"~/Library/Application Support/which-model/config.toml"`.

## 5. Payload map note

`on(event, cb)` callbacks receive the `EngineEventPayloads[event]` value (typed `unknown` at the interface per D00; the mock constructs the exact shapes).

## 6. Event per mutation (mock fires synchronously, once, after mutating `data`)

| Mutation | Event (payload) |
|---|---|
| profiles.save / delete / duplicate | `config:changed` (`{section:'profiles'}`) |
| catalog.saveGroup / duplicateGroup / deleteGroup | `catalog:changed` (`{}`) |
| providers.setEnabled / reorder / setRouteEnabled / setAllRoutes | `config:changed` (`{section:'providers'}`) |
| harnesses.save / delete / setProvider / setAllProviders | `config:changed` (`{section:'harnesses'}`) |
| harnesses.launch, pick.recordPick | `pick:recorded` (`{profile_slug, route_key}`) |
| favourites.pin / unpin | `config:changed` (`{section:'favourites'}`) |
| settings.set | `settings:changed` (the new `GUISettings`) |
| usage.snapshots(force=true), usage.setMode / setBackend | `usage:updated` (`{}`) |

Reads and `window.*` fire nothing. Idempotent no-ops (unpin absent, setEnabled same value) still fire.

## 7. Mock scoring formula (port faithfully; benchScore hashing dropped per SPEC Decisions)

```
benchScore(m, b)  = m.groupScores[HOME(b)] ?? 3.7      // HOME(b) = first §4.3 group listing b
groupScore(m, g)  = mean(benchScore(m, b) for b in g.benchmarks); 3.5 if empty
coreRatio(m, p)   = Σ(w·m.core[k]) / Σ(w·5)  over tier1 keys with w>0; 0.7 if none
taskRatio(m, p)   = Σ(w·groupScore(m, g)) / Σ(w·5) over data.groups with w>0; 0.7 if none
score(m, p)       = 100 · (cs · coreRatio + (1−cs) · taskRatio),  cs = core_share/100
```
`BenchmarkDetail` for name `b`: `note:""`, `groups` = §4.3 slugs listing `b`; one row per model (its own reasoning): `value = round2(benchScore·20)`, `norm = round(value/maxValue·100)`, rows norm desc. `GroupBenchmark`: `covered=8`, `coverage_total=8` for every benchmark.

## 8. Test fixtures (`src/__tests__/`; vitest; write first, red → green)

- **types.test.ts** — compile-time: assigns literal objects with exact snake_case keys to each DTO type; `@ts-expect-error` for a camelCase key and a missing required key; `RankRequest` valid without `overrides`; `ProviderInfo.session` accepts `null`.
- **mock.test.ts** —
  - CRUD round-trips: `profiles.save` (new custom) → `list` contains it; `duplicate('research')` → slug `research_copy`; duplicate again → `research_copy_2`; `delete` removes.
  - EngineError codes: save/delete builtin profile → `builtin_readonly`; `get('nope')` → `not_found`; `saveGroup(…, renameTo)` onto an existing slug → `conflict`; `favourites.pin('bad key')` → `validation_failed`; all caught values satisfy `isEngineError` with the exact code.
  - Event firing: each §6 row fires exactly once with the mapped payload, synchronously before the promise resolves; unsubscribe stops delivery.
  - Rank determinism: `rank({profile_slug:'balanced_implementation', holds:5})` twice → deep-equal responses; asserts the exact 5-element order and 2dp scores computed from §4/§7 (golden literal in the test); disabling provider `codex` reroutes `gpt-5.6-luna` to `copilot`; disabling a model's only route drops it and decrements `total`.
  - Determinism of clock: launch sets `last_used === MOCK_NOW`; no fixture field ever differs across two fresh `createMockEngineHost()` instances.
  - Overrides: `createMockEngineHost({settings: …})` honours the merge; `rank` with `overrides` DTO ignores the named profile's weights and fires no event.

Verify: `pnpm --filter @which-model/core typecheck && pnpm --filter @which-model/core build && pnpm --filter @which-model/core test`.
