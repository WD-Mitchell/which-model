import { EngineError } from './errors.js'
import type { EngineEvent, EngineEventPayloads } from './events.js'
import type { EngineHost } from './host.js'
import type {
  BenchmarkDetail,
  Favourite,
  GroupDetail,
  GUISettings,
  HarnessInfo,
  ProfileDetail,
  ProviderAccount,
  ProviderDetail,
  ProviderInfo,
  RankedModel,
  RankResponse,
  UsageDTO,
  UsageWindow,
} from './types.js'

// Fixed base clock — the package contains no Date.now() and no randomness.
export const MOCK_NOW = '2026-01-01T12:00:00Z'

export interface MockData {
  profiles: ProfileDetail[]
  models: MockModel[]
  groups: { slug: string; builtin: boolean; benchmarks: string[] }[]
  benchmarks: string[]
  harnesses: HarnessInfo[]
  providers: MockProvider[]
  favourites: string[]
  routesDisabled: string[]
  settings: GUISettings
}

export interface MockModel {
  id: string
  name: string
  reasoning: string
  providers: string[]
  core: { intelligence: number; cost: number; speed: number }
  groupScores: Record<string, number>
}

export interface MockProvider {
  id: string
  on: boolean
  priority: number
  auth: string
  limits: string
  session: number | null
  weekly: number | null
  monthly: number | null
  credits: string
  resets: string
}

declare function structuredClone<T>(value: T): T
const clone = <T>(v: T): T => structuredClone(v)
const round2 = (x: number): number => Math.round(x * 100) / 100

// ---------------------------------------------------------------------------
// Fixtures (U01 CONTRACTS §4)
// ---------------------------------------------------------------------------

const GROUP_SLUGS = [
  'software_engineering',
  'reasoning',
  'knowledge',
  'research',
  'instruction_following',
  'agentic_tools',
  'evidence_capture',
  'ui_visual',
  'security',
  'data_ml',
  'finance',
] as const

// Verbatim from the mockup's GROUP_DEFS (demo.dc.html).
const GROUP_DEFS: { slug: string; benchmarks: string[] }[] = [
  { slug: 'software_engineering', benchmarks: ['SWE-Bench Verified', 'SWE-Bench Pro', 'SWE-Bench Multilingual', 'SWE-Bench Multimodal', 'DeepSWE', 'Terminal-Bench', 'Terminal-Bench Hard', 'Aider Polyglot', 'SciCode', 'SWE-Atlas Codebase QnA', 'SWE-Atlas Test Writing', 'SWE-Atlas Refactoring', 'FrontierCode', 'FrontierSWE', 'NL2Repo', 'Program Bench', 'SWE Marathon', 'LiveCodeBench', 'LiveCodeBench Pro', 'MCP Atlas', 'Artificial Analysis Coding Index', 'Artificial Analysis Coding Agent Index', 'Toolathlon', 'AutomationBench'] },
  { slug: 'reasoning', benchmarks: ['GPQA Diamond', 'FrontierMath', 'ARC-AGI-2', 'AIME', 'HMMT'] },
  { slug: 'knowledge', benchmarks: ["Humanity's Last Exam", 'MMLU-Pro', 'MMMU Pro'] },
  { slug: 'research', benchmarks: ['BrowseComp', 'DeepSearchQA', 'WideSearch'] },
  { slug: 'instruction_following', benchmarks: ['IFBench', 'IFEval'] },
  { slug: 'agentic_tools', benchmarks: ['Terminal-Bench', 'Toolathlon', 'MCP Atlas', 'OSWorld-Verified'] },
  { slug: 'evidence_capture', benchmarks: ['OSWorld-Verified', 'Toolathlon', 'MCP Atlas', 'MMMU Pro'] },
  { slug: 'ui_visual', benchmarks: ['MMMU Pro', 'BabyVision', 'OmniDocBench', 'OSWorld-Verified'] },
  { slug: 'security', benchmarks: ['CyberGym', 'CTI-REALM'] },
  { slug: 'data_ml', benchmarks: ['DSBench-FullStack', 'DSBench-Hard', 'MLE-Bench', 'SpreadsheetBench'] },
  { slug: 'finance', benchmarks: ['Finance Agent', 'FinanceAgent', 'τ3 Banking', 'GDPval', 'GDPval-AA'] },
]

// HOME(b) = first §4.3 (builtin) group listing b.
const HOME: Record<string, string> = {}
for (const g of GROUP_DEFS) {
  for (const b of g.benchmarks) {
    if (!(b in HOME)) HOME[b] = g.slug
  }
}

// Verbatim from the mockup's ALL_BENCH, same order.
const ALL_BENCH: string[] = ['AA-Briefcase', 'AIME', 'ARC-AGI-1', 'ARC-AGI-2', 'ARC-AGI-3', "Agents' Last Exam", 'Aider Polyglot', 'Artificial Analysis Coding Agent Index', 'Artificial Analysis Coding Index', 'Artificial Analysis Intelligence Index', 'AutomationBench', 'BabyVision', 'BrowseComp', 'CTI-REALM', 'CharXiv Reasoning', 'ClawEval', 'CritPt', 'CyberGym', 'DSBench-FullStack', 'DSBench-Hard', 'DeepSWE', 'DeepSearchQA', 'FORTE', 'Finance Agent', 'FinanceAgent', 'Frontier-Bench', 'FrontierCode', 'FrontierMath', 'FrontierSWE', 'GDM-MRCR', 'GDPval', 'GDPval-AA', 'GPQA', 'GPQA Diamond', 'GeneBench', 'Graphwalks', 'HMMT', 'HealthBench Professional', "Humanity's Last Exam", 'IFBench', 'IFEval', 'IMOAnswerBench', 'JobBench', 'Kimi Claw 24/7 Bench', 'Kimi Code Bench', 'LiveCodeBench', 'LiveCodeBench Pro', 'Long Context Reasoning', 'MCP Atlas', 'MCP Mark Verified', 'MLE-Bench', 'MLS-Bench-Lite', 'MMLU', 'MMLU-Pro', 'MMMU Pro', 'MRCRv2', 'NL2Repo', 'OSWorld', 'OSWorld-Verified', 'OmniDocBench', 'OpenAI MRCR', 'PostTrainBench', 'Program Bench', 'SWE Bench Pro', 'SWE Marathon', 'SWE-Atlas Codebase QnA', 'SWE-Atlas Refactoring', 'SWE-Atlas Test Writing', 'SWE-Bench Multilingual', 'SWE-Bench Multimodal', 'SWE-Bench Pro', 'SWE-Bench Verified', 'SciCode', 'SpreadsheetBench', 'Terminal Bench 2.0', 'Terminal-Bench', 'Terminal-Bench 2.1', 'Terminal-Bench Hard', 'Tool-Decathlon', 'Toolathlon', 'Toolathlon-Verified', 'WideSearch', 'ZeroBench', 'τ3 Banking', 'τ²-Bench Telecom', 'τ³-Telecom']

function mkModel(
  name: string,
  id: string,
  reasoning: string,
  providers: string[],
  [intelligence, cost, speed]: [number, number, number],
  groupValues: number[],
): MockModel {
  const groupScores: Record<string, number> = {}
  GROUP_SLUGS.forEach((slug, i) => {
    groupScores[slug] = groupValues[i]!
  })
  return { id, name, reasoning, providers, core: { intelligence, cost, speed }, groupScores }
}

function seedModels(): MockModel[] {
  return [
    mkModel('GPT-5.6 Luna', 'gpt-5.6-luna', 'max', ['codex', 'copilot'], [5.0, 3.0, 3.5], [4.9, 4.6, 4.6, 4.8, 4.4, 4.7, 4.4, 4.2, 4.0, 4.5, 4.3]),
    mkModel('Claude Opus 5', 'claude-opus-5', 'max', ['claude'], [4.9, 2.6, 3.2], [4.8, 4.8, 4.5, 4.6, 4.7, 4.9, 4.7, 4.6, 4.5, 4.4, 4.3]),
    mkModel('GPT-5.6 Sol', 'gpt-5.6-sol', 'high', ['copilot', 'codex'], [4.4, 4.0, 4.4], [4.3, 4.0, 4.2, 4.1, 4.2, 4.0, 4.0, 3.8, 3.9, 4.0, 3.9]),
    mkModel('Claude Sonnet 5.2', 'claude-sonnet-5.2', 'high', ['claude', 'copilot'], [4.2, 4.4, 4.6], [4.5, 4.1, 4.0, 3.9, 4.6, 4.4, 4.3, 4.4, 4.2, 4.0, 4.0]),
    mkModel('Gemini 3.5 Ultra', 'gemini-3.5-ultra', 'max', ['cursor'], [4.7, 3.4, 3.8], [4.4, 4.5, 4.8, 4.7, 4.0, 4.2, 4.2, 4.3, 4.1, 4.4, 4.1]),
    mkModel('Grok 5 Fast', 'grok-5-fast', 'medium', ['cursor', 'copilot'], [3.8, 4.7, 5.0], [4.0, 3.5, 3.6, 3.4, 3.9, 3.8, 3.6, 3.6, 3.4, 3.5, 3.4]),
    mkModel('Qwen 3.5 Max', 'qwen-3.5-max', 'medium', ['cursor'], [4.0, 4.9, 4.2], [4.1, 3.8, 4.0, 3.8, 3.7, 3.6, 3.6, 3.4, 3.5, 3.7, 3.5]),
    mkModel('Llama 5 405B', 'llama-5-405b', 'low', ['copilot'], [3.5, 5.0, 4.0], [3.5, 3.2, 3.6, 3.2, 3.4, 3.2, 3.2, 3.0, 3.2, 3.3, 3.1]),
  ]
}

function mkProfile(
  slug: string,
  name: string,
  coreShare: number,
  [intelligence, cost, speed]: [number, number, number],
  tier2: Record<string, number>,
  picks: number,
  lastUsed: string,
): ProfileDetail {
  return {
    slug,
    name,
    builtin: true,
    core_share: coreShare,
    tier1_weights: { intelligence, cost, speed },
    tier2_weights: tier2,
    picks,
    last_used: lastUsed,
  }
}

const COMPLEXITY_SCALE = [
  'simple_action_execution',
  'simple_implementation',
  'balanced_implementation',
  'research',
  'planning',
]

function seedProfiles(): ProfileDetail[] {
  return [
    mkProfile('simple_action_execution', 'Simple Action', 75, [2, 5, 5], { instruction_following: 4, agentic_tools: 3 }, 312, '2026-01-01T11:48:00Z'),
    mkProfile('simple_implementation', 'Simple Implementation', 60, [4, 4, 3], { software_engineering: 4, instruction_following: 3, agentic_tools: 3 }, 1284, '2026-01-01T11:00:00Z'),
    mkProfile('balanced_implementation', 'Balanced Implementation', 70, [4, 3, 3], { software_engineering: 5, agentic_tools: 4, instruction_following: 3 }, 866, '2025-12-31T12:00:00Z'),
    mkProfile('research', 'Research', 60, [4, 4, 2], { research: 5, knowledge: 4, agentic_tools: 3 }, 174, '2025-12-29T12:00:00Z'),
    mkProfile('planning', 'Planning', 60, [5, 2, 2], { reasoning: 5, research: 4, knowledge: 3 }, 121, ''),
    mkProfile('review', 'Review', 65, [4, 3, 3], { instruction_following: 5, security: 3 }, 58, ''),
    mkProfile('ui_ux', 'UI / UX', 60, [4, 3, 3], { ui_visual: 5, software_engineering: 4 }, 43, ''),
    mkProfile('research_fast', 'Research (fast)', 60, [3, 4, 5], { research: 5, knowledge: 3 }, 19, ''),
  ]
}

const PROVIDER_IDS = ['claude', 'codex', 'copilot', 'cursor']

function mkHarness(
  slug: string,
  name: string,
  command: string,
  installed: boolean,
  providersOn: string[],
): HarnessInfo {
  const providers: Record<string, boolean> = {}
  for (const id of PROVIDER_IDS) providers[id] = providersOn.includes(id)
  return { slug, name, command, builtin: true, installed, providers }
}

function seedHarnesses(): HarnessInfo[] {
  return [
    mkHarness('claude', 'Claude Code', 'claude --model {model_id} --reasoning {reasoning}', true, ['claude', 'codex', 'copilot']),
    mkHarness('codex', 'Codex CLI', 'codex -m {model_id} -c reasoning={reasoning}', true, ['codex', 'copilot']),
    mkHarness('copilot', 'Copilot CLI', 'copilot --model {model_id}', true, ['copilot', 'cursor']),
    mkHarness('cursor', 'Cursor', 'cursor --model {model_id}', false, ['cursor']),
  ]
}

function seedProviders(): MockProvider[] {
  return [
    { id: 'claude', on: true, priority: 1, auth: 'oauth', limits: 'session 42% · weekly 18%', session: 42, weekly: 18, monthly: 54, credits: 'max 20× plan', resets: 'session in 2h 40m' },
    { id: 'codex', on: true, priority: 2, auth: 'oauth', limits: 'session 12% · weekly 31% · 340 credits', session: 12, weekly: 31, monthly: 44, credits: '340 credits left', resets: 'weekly on Mon' },
    { id: 'copilot', on: true, priority: 3, auth: 'device flow', limits: 'monthly 1200 of 4800', session: 8, weekly: 25, monthly: 25, credits: '1200 of 4800 premium', resets: 'monthly on the 1st' },
    { id: 'cursor', on: false, priority: 4, auth: 'via codexbar', limits: 'not enabled', session: null, weekly: null, monthly: null, credits: 'no plan detected', resets: '—' },
  ]
}

function seedSettings(): GUISettings {
  return {
    layout: 'carousel',
    default_tab: 'profiles',
    weight_control: 'slider',
    holds: 5,
    shortcut: 'alt+space',
    show_menu_bar_icon: true,
    launch_at_login: false,
    copy_command_instead: false,
    close_popover_after_launch: true,
    auto_update: true,
    auto_update_frequency: 'daily',
    mcp_server: false,
    claude_md_hint: false,
    shell_alias: false,
    use_keychain: true,
    config_path: '~/Library/Application Support/which-model/config.toml',
    version: 'which-model dev (commit unknown, built unknown)',
  }
}

function seedData(): MockData {
  return {
    profiles: seedProfiles(),
    models: seedModels(),
    groups: GROUP_DEFS.map((g) => ({ slug: g.slug, builtin: true, benchmarks: [...g.benchmarks] })),
    benchmarks: [...ALL_BENCH],
    harnesses: seedHarnesses(),
    providers: seedProviders(),
    favourites: [],
    routesDisabled: [],
    settings: seedSettings(),
  }
}

// ---------------------------------------------------------------------------
// Route keys (D00 CONTRACTS §1)
// ---------------------------------------------------------------------------

const ROUTE_KEY_RE = /^([a-z0-9_]+)\/([A-Za-z0-9._-]+)@(minimal|low|medium|high|xhigh|max|default)$/
const SLUG_RE = /^[a-z0-9_]+$/

interface RouteKeyParts {
  provider: string
  modelId: string
  reasoning: string
}

function parseRouteKey(key: string): RouteKeyParts {
  const m = ROUTE_KEY_RE.exec(key)
  if (!m) {
    throw new EngineError('validation_failed', `invalid route key "${key}"`)
  }
  return { provider: m[1]!, modelId: m[2]!, reasoning: m[3]! }
}

function formatRouteKey(provider: string, modelId: string, reasoning: string): string {
  return `${provider}/${modelId}@${reasoning}`
}

// ---------------------------------------------------------------------------
// Scoring (U01 CONTRACTS §7)
// ---------------------------------------------------------------------------

function benchScore(m: MockModel, bench: string): number {
  const home = HOME[bench]
  return (home !== undefined ? m.groupScores[home] : undefined) ?? 3.7
}

function groupScore(m: MockModel, benchmarks: string[]): number {
  if (benchmarks.length === 0) return 3.5
  let sum = 0
  for (const b of benchmarks) sum += benchScore(m, b)
  return sum / benchmarks.length
}

function scoreModel(data: MockData, m: MockModel, p: ProfileDetail): number {
  let coreNum = 0
  let coreDen = 0
  for (const [key, w] of Object.entries(p.tier1_weights)) {
    if (w > 0) {
      coreNum += w * (m.core[key as keyof MockModel['core']] ?? 0)
      coreDen += w * 5
    }
  }
  const coreRatio = coreDen > 0 ? coreNum / coreDen : 0.7

  let taskNum = 0
  let taskDen = 0
  for (const g of data.groups) {
    const w = p.tier2_weights[g.slug] ?? 0
    if (w > 0) {
      taskNum += w * groupScore(m, g.benchmarks)
      taskDen += w * 5
    }
  }
  const taskRatio = taskDen > 0 ? taskNum / taskDen : 0.7

  const cs = p.core_share / 100
  return 100 * (cs * coreRatio + (1 - cs) * taskRatio)
}

// ---------------------------------------------------------------------------
// Mock host
// ---------------------------------------------------------------------------

// Per-provider accounts for browser mode; the real store is config.toml.
const mockAccounts: Record<string, ProviderAccount[]> = {
  claude: [{ name: 'Work', kind: 'oauth', ref: '~/.claude/.credentials.json' }],
  // copilot needs a signed-out oauth row so the browser mock can exercise
  // the device-flow sign-in modal (empty ref renders the "Sign in…" button).
  copilot: [{ name: 'GitHub', kind: 'oauth', ref: '' }],
}

export function createMockEngineHost(
  overrides?: Partial<MockData>,
): EngineHost & { data: MockData } {
  const data: MockData = { ...seedData(), ...(overrides ? clone(overrides) : {}) }

  let usageMode = 'auto'
  let usageBackend = 'native'

  const listeners = new Map<EngineEvent, Set<(payload: unknown) => void>>()

  function emit<E extends EngineEvent>(event: E, payload: EngineEventPayloads[E]): void {
    const set = listeners.get(event)
    if (!set) return
    // Iterate a snapshot so unsubscribing during dispatch is safe.
    for (const cb of [...set]) {
      if (set.has(cb)) cb(clone(payload))
    }
  }

  const notFound = (kind: string, id: string): EngineError =>
    new EngineError('not_found', `${kind} "${id}" does not exist`)

  function requireProfile(slug: string): ProfileDetail {
    const p = data.profiles.find((x) => x.slug === slug)
    if (!p) throw notFound('profile', slug)
    return p
  }

  function requireGroup(slug: string): { slug: string; builtin: boolean; benchmarks: string[] } {
    const g = data.groups.find((x) => x.slug === slug)
    if (!g) throw notFound('group', slug)
    return g
  }

  function requireProvider(id: string): MockProvider {
    const p = data.providers.find((x) => x.id === id)
    if (!p) throw notFound('provider', id)
    return p
  }

  function requireHarness(slug: string): HarnessInfo {
    const h = data.harnesses.find((x) => x.slug === slug)
    if (!h) throw notFound('harness', slug)
    return h
  }

  function freeSlug(base: string, taken: (slug: string) => boolean): string {
    let candidate = `${base}_copy`
    for (let n = 2; taken(candidate); n++) candidate = `${base}_copy_${n}`
    return candidate
  }

  function routeDisabled(provider: string, modelId: string, reasoning: string): boolean {
    return data.routesDisabled.includes(formatRouteKey(provider, modelId, reasoning))
  }

  function providersByPriority(): MockProvider[] {
    return [...data.providers].sort((a, b) => a.priority - b.priority)
  }

  // Candidate route for a model: first enabled provider in priority order that
  // the model lists and whose route is not disabled; null when none.
  function routeFor(m: MockModel): string | null {
    for (const p of providersByPriority()) {
      if (p.on && m.providers.includes(p.id) && !routeDisabled(p.id, m.id, m.reasoning)) {
        return p.id
      }
    }
    return null
  }

  function computeRank(profile: ProfileDetail, holds: number): RankResponse {
    const scored: { m: MockModel; provider: string; score: number }[] = []
    for (const m of data.models) {
      const provider = routeFor(m)
      if (provider === null) continue
      scored.push({ m, provider, score: round2(scoreModel(data, m, profile)) })
    }
    scored.sort((a, b) => b.score - a.score || (a.m.id < b.m.id ? -1 : a.m.id > b.m.id ? 1 : 0))
    const total = scored.length
    const candidates: RankedModel[] = scored.slice(0, holds).map((c, i) => ({
      rank: i + 1,
      model_id: c.m.id,
      model_name: c.m.name,
      provider: c.provider,
      reasoning: c.m.reasoning,
      score: c.score,
      route_key: formatRouteKey(c.provider, c.m.id, c.m.reasoning),
    }))
    return { candidates, total }
  }

  function recordPickInternal(profileSlug: string, routeKey: string): void {
    parseRouteKey(routeKey)
    const profile = data.profiles.find((p) => p.slug === profileSlug)
    if (profile) {
      profile.picks += 1
      profile.last_used = MOCK_NOW
    }
    emit('pick:recorded', { profile_slug: profileSlug, route_key: routeKey })
  }

  function groupDetailFor(g: { slug: string; builtin: boolean; benchmarks: string[] }): GroupDetail {
    return {
      slug: g.slug,
      builtin: g.builtin,
      benchmarks: data.benchmarks.map((name) => ({
        name,
        on: g.benchmarks.includes(name),
        covered: 8,
        coverage_total: 8,
      })),
    }
  }

  const host: EngineHost & { data: MockData } = {
    data,

    profiles: {
      async list() {
        return clone(data.profiles)
      },
      async get(slug) {
        return clone(requireProfile(slug))
      },
      async save(p) {
        if (!SLUG_RE.test(p.slug)) {
          throw new EngineError('validation_failed', `invalid profile slug "${p.slug}"`)
        }
        const existing = data.profiles.find((x) => x.slug === p.slug)
        if (existing?.builtin) {
          throw new EngineError('builtin_readonly', `profile "${p.slug}" is built-in and read-only`)
        }
        const saved = { ...clone(p), builtin: false }
        if (existing) {
          data.profiles[data.profiles.indexOf(existing)] = saved
        } else {
          data.profiles.push(saved)
        }
        emit('config:changed', { section: 'profiles' })
      },
      async duplicate(slug) {
        const src = requireProfile(slug)
        const copy: ProfileDetail = {
          ...clone(src),
          slug: freeSlug(src.slug, (s) => data.profiles.some((p) => p.slug === s)),
          name: `${src.name} copy`,
          builtin: false,
          picks: 0,
          last_used: '',
        }
        data.profiles.push(copy)
        emit('config:changed', { section: 'profiles' })
        return clone(copy)
      },
      async delete(slug) {
        const p = requireProfile(slug)
        if (p.builtin) {
          throw new EngineError('builtin_readonly', `profile "${slug}" is built-in and read-only`)
        }
        data.profiles.splice(data.profiles.indexOf(p), 1)
        emit('config:changed', { section: 'profiles' })
      },
      async complexityScale() {
        return [...COMPLEXITY_SCALE]
      },
    },

    pick: {
      async rank(req) {
        const profile = req.overrides ?? requireProfile(req.profile_slug)
        const holds = req.holds !== 0 ? req.holds : data.settings.holds
        if (holds !== 3 && holds !== 5 && holds !== 10) {
          throw new EngineError('validation_failed', `holds ${holds} must be 3, 5 or 10`)
        }
        return clone(computeRank(profile, holds))
      },
      async recordPick(profileSlug, routeKey) {
        recordPickInternal(profileSlug, routeKey)
      },
      async catalogLine() {
        return {
          models: data.models.length,
          providers_on: data.providers.filter((p) => p.on).length,
          harnesses: data.harnesses.length,
        }
      },
    },

    catalog: {
      async benchmarks() {
        return clone(data.benchmarks)
      },
      async benchmarkDetail(name) {
        if (!data.benchmarks.includes(name)) throw notFound('benchmark', name)
        const rows = data.models.map((m) => {
          const value = round2(benchScore(m, name) * 20)
          return { model: m.name, reasoning: m.reasoning, value }
        })
        const maxValue = Math.max(...rows.map((r) => r.value))
        const withNorm: BenchmarkDetail['rows'] = rows.map((r) => ({
          ...r,
          norm: Math.round((r.value / maxValue) * 100),
        }))
        withNorm.sort((a, b) => b.norm - a.norm || (a.model < b.model ? -1 : a.model > b.model ? 1 : 0))
        return {
          name,
          note: '',
          groups: data.groups.filter((g) => g.benchmarks.includes(name)).map((g) => g.slug),
          rows: withNorm,
        }
      },
      async groups() {
        return data.groups.map((g) => ({
          slug: g.slug,
          builtin: g.builtin,
          benchmark_count: g.benchmarks.length,
          in_profiles: data.profiles.filter((p) => (p.tier2_weights[g.slug] ?? 0) > 0).length,
        }))
      },
      async groupDetail(slug) {
        return groupDetailFor(requireGroup(slug))
      },
      async saveGroup(slug, benchmarks, renameTo) {
        const g = requireGroup(slug)
        if (g.builtin) {
          throw new EngineError('builtin_readonly', `group "${slug}" is built-in and read-only`)
        }
        for (const b of benchmarks) {
          if (!data.benchmarks.includes(b)) {
            throw new EngineError('validation_failed', `unknown benchmark "${b}"`)
          }
        }
        if (renameTo !== undefined && renameTo !== slug) {
          if (!SLUG_RE.test(renameTo)) {
            throw new EngineError('validation_failed', `invalid group slug "${renameTo}"`)
          }
          if (data.groups.some((x) => x.slug === renameTo)) {
            throw new EngineError('conflict', `group "${renameTo}" already exists`)
          }
          g.slug = renameTo
        }
        g.benchmarks = [...benchmarks]
        emit('catalog:changed', {})
      },
      async duplicateGroup(slug) {
        const src = requireGroup(slug)
        const copy = {
          slug: freeSlug(src.slug, (s) => data.groups.some((g) => g.slug === s)),
          builtin: false,
          benchmarks: [...src.benchmarks],
        }
        data.groups.push(copy)
        emit('catalog:changed', {})
        return groupDetailFor(copy)
      },
      async deleteGroup(slug) {
        const g = requireGroup(slug)
        if (g.builtin) {
          throw new EngineError('builtin_readonly', `group "${slug}" is built-in and read-only`)
        }
        data.groups.splice(data.groups.indexOf(g), 1)
        emit('catalog:changed', {})
      },
    },

    providers: {
      async add(_id) {},
      async addable() {
        return ['google', 'mistral', 'xai']
      },
      async delete(_id) {},
      async duplicate(id) {
        return `${id}_2`
      },
      async setAccounts(id, accounts) {
        mockAccounts[id] = accounts.map((a) => ({ ...a }))
        emit('config:changed', { section: 'providers' })
      },
      async list() {
        return providersByPriority().map((p): ProviderInfo => {
          const models = data.models.filter((m) => m.providers.includes(p.id))
          return {
            id: p.id,
            enabled: p.on,
            priority: p.priority,
            auth: p.auth,
            limits_line: p.limits,
            routes_on: models.filter((m) => !routeDisabled(p.id, m.id, m.reasoning)).length,
            routes_total: models.length,
            session: p.session,
            weekly: p.weekly,
            monthly: p.monthly,
            credits: p.credits,
            resets: p.resets,
            accounts: (mockAccounts[p.id] ?? []).length,
            // Mirrors the real registry: these three ship usage adapters.
            builtin: ['claude', 'codex', 'copilot'].includes(p.id),
          }
        })
      },
      async setEnabled(id, on) {
        requireProvider(id).on = on
        emit('config:changed', { section: 'providers' })
      },
      async reorder(orderedIds) {
        for (const id of orderedIds) requireProvider(id)
        if (new Set(orderedIds).size !== data.providers.length) {
          throw new EngineError('validation_failed', 'reorder must list every provider exactly once')
        }
        orderedIds.forEach((id, i) => {
          requireProvider(id).priority = i + 1
        })
        emit('config:changed', { section: 'providers' })
      },
      async detail(id) {
        const p = requireProvider(id)
        const detail: ProviderDetail = {
          id: p.id,
          accounts: mockAccounts[p.id] ?? [],
          builtin: ['claude', 'codex', 'copilot'].includes(p.id),
          models: data.models
            .filter((m) => m.providers.includes(p.id))
            .map((m) => ({
              model_id: m.id,
              model_name: m.name,
              levels: [
                {
                  reasoning: m.reasoning,
                  enabled: !routeDisabled(p.id, m.id, m.reasoning),
                  default: true,
                },
              ],
            })),
        }
        return detail
      },
      async setRouteEnabled(id, modelId, reasoning, on) {
        requireProvider(id)
        const key = formatRouteKey(id, modelId, reasoning)
        parseRouteKey(key)
        const i = data.routesDisabled.indexOf(key)
        if (on && i !== -1) data.routesDisabled.splice(i, 1)
        if (!on && i === -1) data.routesDisabled.push(key)
        emit('config:changed', { section: 'providers' })
      },
      async setAllRoutes(id, on) {
        requireProvider(id)
        for (const m of data.models.filter((x) => x.providers.includes(id))) {
          const key = formatRouteKey(id, m.id, m.reasoning)
          const i = data.routesDisabled.indexOf(key)
          if (on && i !== -1) data.routesDisabled.splice(i, 1)
          if (!on && i === -1) data.routesDisabled.push(key)
        }
        emit('config:changed', { section: 'providers' })
      },
    },

    harnesses: {
      async list() {
        return clone(data.harnesses)
      },
      async save(h) {
        if (!SLUG_RE.test(h.slug)) {
          throw new EngineError('validation_failed', `invalid harness slug "${h.slug}"`)
        }
        for (const [, token] of h.command.matchAll(/\{([^}]*)\}/g)) {
          if (token !== 'model_id' && token !== 'reasoning') {
            throw new EngineError('validation_failed', `unknown template token "{${token}}" in command`)
          }
        }
        const existing = data.harnesses.find((x) => x.slug === h.slug)
        if (existing?.builtin) {
          throw new EngineError('builtin_readonly', `harness "${h.slug}" is built-in and read-only`)
        }
        const saved = { ...clone(h), builtin: false }
        if (existing) {
          data.harnesses[data.harnesses.indexOf(existing)] = saved
        } else {
          data.harnesses.push(saved)
        }
        emit('config:changed', { section: 'harnesses' })
      },
      async delete(slug) {
        const h = requireHarness(slug)
        if (h.builtin) {
          throw new EngineError('builtin_readonly', `harness "${slug}" is built-in and read-only`)
        }
        data.harnesses.splice(data.harnesses.indexOf(h), 1)
        emit('config:changed', { section: 'harnesses' })
      },
      async setProvider(slug, provider, on) {
        const h = requireHarness(slug)
        requireProvider(provider)
        h.providers[provider] = on
        emit('config:changed', { section: 'harnesses' })
      },
      async setAllProviders(slug, on) {
        const h = requireHarness(slug)
        for (const id of Object.keys(h.providers)) h.providers[id] = on
        emit('config:changed', { section: 'harnesses' })
      },
      async launch(slug, routeKey, profileSlug) {
        const parts = parseRouteKey(routeKey)
        const h = requireHarness(slug)
        const command = h.command
          .replaceAll('{model_id}', parts.modelId)
          .replaceAll('{reasoning}', parts.reasoning)
        recordPickInternal(profileSlug, routeKey)
        return { copied: data.settings.copy_command_instead, command }
      },
    },

    usage: {
      async snapshots(force) {
        const snaps = providersByPriority()
          .filter((p) => p.on)
          .map((p): UsageDTO => {
            const windows: UsageWindow[] = []
            const add = (id: string, label: string, used: number | null): void => {
              if (used !== null) {
                windows.push({ id, label, used_percent: used, reset_hint: '', unlimited: false })
              }
            }
            add('session', 'Session', p.session)
            add('weekly', 'Weekly', p.weekly)
            add('monthly', 'Monthly', p.monthly)
            return {
              provider: p.id,
              plan: p.credits,
              auth: p.auth,
              confidence: 'live',
              stale: false,
              windows,
              credits: p.credits,
              resets: p.resets,
              failure: '',
            }
          })
        if (force) emit('usage:updated', {})
        return snaps
      },
      async setMode(mode) {
        usageMode = mode
        emit('usage:updated', {})
      },
      async setBackend(backend) {
        usageBackend = backend
        emit('usage:updated', {})
      },
      async mode() {
        return { mode: usageMode, backend: usageBackend }
      },
    },

    favourites: {
      async list() {
        return data.favourites.map((key): Favourite => {
          const parts = parseRouteKey(key)
          const model = data.models.find((m) => m.id === parts.modelId)
          const provider = data.providers.find((p) => p.id === parts.provider)
          return {
            route_key: key,
            model_name: model?.name ?? parts.modelId,
            route_label: provider
              ? `${parts.provider} · ${parts.reasoning}`
              : `no provider · ${parts.reasoning}`,
            in_range:
              model !== undefined &&
              provider !== undefined &&
              provider.on &&
              model.providers.includes(parts.provider) &&
              model.reasoning === parts.reasoning &&
              !data.routesDisabled.includes(key),
          }
        })
      },
      async pin(routeKey) {
        parseRouteKey(routeKey)
        if (!data.favourites.includes(routeKey)) data.favourites.push(routeKey)
        emit('config:changed', { section: 'favourites' })
      },
      async unpin(routeKey) {
        parseRouteKey(routeKey)
        const i = data.favourites.indexOf(routeKey)
        if (i !== -1) data.favourites.splice(i, 1)
        emit('config:changed', { section: 'favourites' })
      },
    },

    settings: {
      async get() {
        return clone(data.settings)
      },
      async set(s) {
        data.settings = clone(s)
        emit('settings:changed', data.settings)
      },
      async shellSnippets() {
        const profile =
          data.profiles.find((p) => p.slug === 'balanced_implementation') ?? data.profiles[0]
        let preview = ''
        if (profile) {
          const top = computeRank(profile, 1).candidates[0]
          if (top) {
            preview = `$ wm ${profile.slug}  →  ${top.model_id}  (${top.provider})`
          }
        }
        return {
          alias: 'alias wm="which-model pick"',
          claude_md: 'Before launching a coding agent, run `wm <profile>` to pick the model.',
          preview,
        }
      },
    },

    signin: {
      async start(provider) {
        if (provider !== 'copilot') {
          throw new EngineError('validation_failed', `sign-in for ${provider} is not supported`)
        }
        return { verification_uri: 'https://github.com/login/device', user_code: 'WDML-MOCK' }
      },
      async confirm() {},
      async cancel() {},
    },

    window: {
      async openSettings() {},
      async closeSettings() {},
      async hidePopover() {},
      async quit() {},
      async copyToClipboard(_text) {},
      async setPopoverHeight(_height) {},
      async setTrayPick(_profileName, _modelName, _reasoning, _provider) {},
    },

    on(event, cb) {
      let set = listeners.get(event)
      if (!set) {
        set = new Set()
        listeners.set(event, set)
      }
      set.add(cb)
      return () => {
        set.delete(cb)
      }
    },
  }

  return host
}
