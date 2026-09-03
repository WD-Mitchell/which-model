import { describe, expect, it } from 'vitest'
import type {
  BenchmarkDetail,
  BenchRow,
  CatalogModel,
  CatalogModelDetail,
  CatalogSummary,
  ErrorDTO,
  Favourite,
  GroupBenchmark,
  GroupDetail,
  GroupSummary,
  GUISettings,
  HarnessInfo,
  LaunchResult,
  ModelScoreDetail,
  ProfileDetail,
  ProfileStats,
  ProfileSummary,
  ProviderDetail,
  ProviderInfo,
  ProviderModel,
  RankedModel,
  RankRequest,
  RankResponse,
  RouteLevel,
  ShellSnippets,
  UsageDTO,
  UsageWindow,
} from '../index.js'

// Compile-level assertions: literal objects with the exact snake_case keys
// assign to each DTO type; wrong-case or missing keys are compile errors.

const profile: ProfileSummary = {
  slug: 'balanced_implementation',
  name: 'Balanced Implementation',
  builtin: true,
  core_share: 70,
  tier1_weights: { intelligence: 4, cost: 3, speed: 3 },
  tier2_weights: { software_engineering: 5 },
  picks: 866,
  last_used: '2025-12-31T12:00:00Z',
}

// ProfileDetail is a type alias of ProfileSummary.
const detail: ProfileDetail = profile

// @ts-expect-error — camelCase key (coreShare) is rejected
const badCase: ProfileSummary = { slug: 's', name: 'n', builtin: false, core_share: 70, coreShare: 70, tier1_weights: {}, tier2_weights: {}, picks: 0, last_used: '' }

// @ts-expect-error — missing required key (core_share) is rejected
const missingKey: ProfileSummary = { slug: 's', name: 'n', builtin: false, tier1_weights: {}, tier2_weights: {}, picks: 0, last_used: '' }

// RankRequest is valid without `overrides` (optional + nullable)...
const rankReq: RankRequest = { profile_slug: 'research', holds: 5 }
// ...and accepts it as a ProfileDetail or null.
const rankReqWithOverrides: RankRequest = {
  profile_slug: 'research',
  overrides: profile,
  holds: 0,
}
const rankReqNullOverrides: RankRequest = {
  profile_slug: 'research',
  overrides: null,
  holds: 3,
}

const ranked: RankedModel = {
  rank: 1,
  model_id: 'claude-opus-5',
  model_name: 'Claude Opus 5',
  provider: 'claude',
  reasoning: 'max',
  score: 91.23,
  route_key: 'claude/claude-opus-5@max',
}
const rankResp: RankResponse = { candidates: [ranked], total: 8 }

const catalogLine: CatalogSummary = { models: 8, providers_on: 3, harnesses: 4 }

// ProviderInfo.session accepts null (Go *int → number | null).
const provider: ProviderInfo = {
  id: 'cursor',
  enabled: false,
  priority: 4,
  models: 3,
  auth: 'via codexbar',
  limits_line: 'not enabled',
  routes_on: 0,
  routes_total: 3,
  session: null,
  weekly: null,
  monthly: null,
  credits: 'no plan detected',
  resets: '—',
  accounts: 0,
  builtin: false,
}
const providerWithNumbers: ProviderInfo = { ...provider, session: 42, weekly: 18, monthly: 54 }

const level: RouteLevel = { reasoning: 'max', enabled: true, default: true }
const providerModel: ProviderModel = {
  model_id: 'claude-opus-5',
  model_name: 'Claude Opus 5',
  levels: [level],
}
const providerDetail: ProviderDetail = {
  id: 'claude',
  models: [providerModel],
  accounts: [],
  oauth_supported: true,
  builtin: true,
}

const harness: HarnessInfo = {
  slug: 'claude',
  name: 'Claude Code',
  command: 'claude --model {model_id} --reasoning {reasoning}',
  builtin: true,
  installed: true,
  enabled: true,
  providers: { claude: true, codex: true, copilot: true, cursor: false },
}

const launch: LaunchResult = { copied: false, command: 'claude --model claude-opus-5 --reasoning max' }

const usageWindow: UsageWindow = {
  id: 'session',
  label: 'Session',
  used_percent: null,
  reset_hint: 'in 2h 40m',
  unlimited: false,
}
const usage: UsageDTO = {
  provider: 'claude',
  plan: 'max 20× plan',
  auth: 'oauth',
  confidence: 'live',
  stale: false,
  windows: [usageWindow],
  credits: 'max 20× plan',
  resets: 'session in 2h 40m',
  failure: '',
}

const groupBenchmark: GroupBenchmark = {
  name: 'SWE-Bench Verified',
  on: true,
  covered: 8,
  coverage_total: 8,
}
const groupSummary: GroupSummary = {
  slug: 'software_engineering',
  builtin: true,
  benchmark_count: 24,
  in_profiles: 3,
}
const groupDetail: GroupDetail = {
  slug: 'software_engineering',
  builtin: true,
  benchmarks: [groupBenchmark],
}

const benchRow: BenchRow = { model: 'Claude Opus 5', reasoning: 'max', value: 96, norm: 98 }
const benchmarkDetail: BenchmarkDetail = {
  name: 'SWE-Bench Verified',
  note: '',
  groups: ['software_engineering'],
  rows: [benchRow],
}
const modelScoreDetail: ModelScoreDetail = {
  model: 'Claude Opus 5',
  reasoning: 'max',
  rows: [{ name: 'SWE-Bench Verified', value: 96, norm: 100, groups: ['software_engineering'] }],
}
const catalogModel: CatalogModel = {
  model_name: 'Claude Opus 5',
  model_id: 'claude-opus-5',
  reasoning: ['max'],
  intelligence: 100,
  cost: 0,
  speed: 12,
  provider_count: 1,
}

const catalogModelDetail: CatalogModelDetail = {
  model_name: 'Claude Opus 5',
  model_id: 'claude-opus-5',
  reasoning: ['max'],
  intelligence: 100,
  cost: 0,
  speed: 12,
  provider_count: 1,
  in_catalog: true,
  providers: [
    {
      provider: 'claude',
      model_id: 'claude-opus-5',
      reasoning: ['max'],
      route_keys: ['claude/claude-opus-5@max'],
      input_cost_usd_per_m: 15,
      output_cost_usd_per_m: 75,
    },
  ],
}

const favourite: Favourite = {
  route_key: 'claude/claude-opus-5@max',
  model_name: 'Claude Opus 5',
  route_label: 'claude · max',
  in_range: true,
}

const settings: GUISettings = {
  layout: 'carousel',
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
  catalog_repo: 'WD-Mitchell/which-model',
  use_local_aa: false,
  benchmark_check_frequency: '6h',
  aa_api_key: '',
  aa_api_key_set: false,
  config_path: '~/Library/Application Support/which-model/config.toml',
  default_tab: 'profiles',
  version: 'which-model dev',
}

// @ts-expect-error — layout is a closed union, arbitrary strings rejected
const badSettings: GUISettings = { ...settings, layout: 'grid' }

const snippets: ShellSnippets = { alias: 'a', claude_md: 'c', preview: 'p' }
const stats: ProfileStats = { picks: 1, last_used: '' }
const errorDto: ErrorDTO = { code: 'not_found', message: 'no such profile' }

describe('types', () => {
  it('DTO literals compile with exact snake_case keys', () => {
    // Runtime is trivial — the assertions above are compile-level.
    const all = [
      profile, detail, badCase, missingKey, rankReq, rankReqWithOverrides,
      rankReqNullOverrides, ranked, rankResp, catalogLine, provider,
      providerWithNumbers, providerDetail, harness, launch, usage, groupDetail,
      groupSummary, benchmarkDetail, modelScoreDetail, favourite, settings, badSettings,
      snippets, stats, errorDto,
    ]
    expect(all.length).toBeGreaterThan(0)
    expect(provider.session).toBeNull()
    expect(rankReq.overrides).toBeUndefined()
  })
})
