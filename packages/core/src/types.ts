// TS mirrors of the canonical Go DTOs (D00 CONTRACTS §2).
// Field names are the Go JSON tags (snake_case), field-for-field.
// Go `*T` → `T | null`; `*T,omitempty` (only RankRequest.Overrides) → `field?: T | null`.

export interface ProfileSummary {
  description?: string
  evidence_note?: string
  slug: string
  name: string
  builtin: boolean
  core_share: number
  tier1_weights: Record<string, number>
  tier2_weights: Record<string, number>
  picks: number
  last_used: string
}
export type ProfileDetail = ProfileSummary

export interface RankRequest {
  profile_slug: string
  overrides?: ProfileDetail | null
  holds: number
}

export interface RankedModel {
  rank: number
  model_id: string
  model_name: string
  provider: string
  reasoning: string
  score: number
  route_key: string
  intelligence?: number | null
  cost?: number | null
  speed?: number | null
}

export interface RankResponse {
  candidates: RankedModel[]
  total: number
}

export interface CatalogSummary {
  models: number
  providers_on: number
  harnesses: number
}

export interface ProviderInfo {
  id: string
  enabled: boolean
  priority: number
  /** Distinct model IDs available through this provider. */
  models: number
  auth: string
  limits_line: string
  routes_on: number
  routes_total: number
  session: number | null
  weekly: number | null
  monthly: number | null
  credits: string
  resets: string
  /** Configured account count. */
  accounts: number
  /** Ships a usage adapter: cannot be deleted, only disabled. */
  builtin: boolean
}

export interface RouteLevel {
  reasoning: string
  enabled: boolean
  default: boolean
}
export interface ProviderModel {
  model_id: string
  model_name: string
  levels: RouteLevel[]
}
/** One named credential for a provider. `ref` identifies a managed credential,
 *  file, or keychain service — never the secret itself. */
export interface ProviderAccount {
  name: string
  kind: 'oauth' | 'cookie' | 'token'
  ref: string
}

export interface ProviderDetail {
  id: string
  models: ProviderModel[]
  accounts: ProviderAccount[]
  /** True only when which-model can start an interactive OAuth flow. */
  oauth_supported: boolean
  /** Ships through which-model or the configured backend; disable, don't delete. */
  builtin: boolean
}

export interface HarnessInfo {
  slug: string
  name: string
  command: string
  builtin: boolean
  installed: boolean
  enabled: boolean
  providers: Record<string, boolean>
}

export interface LaunchResult {
  copied: boolean
  command: string
}

export interface UsageWindow {
  id: string
  label: string
  used_percent: number | null
  reset_hint: string
  unlimited: boolean
}
export interface UsageDTO {
  provider: string
  plan: string
  auth: string
  confidence: 'live' | 'cached' | 'estimated'
  stale: boolean
  windows: UsageWindow[]
  credits: string
  resets: string
  failure: string
}

export interface GroupBenchmark {
  name: string
  on: boolean
  covered: number
  coverage_total: number
}
export interface GroupSummary {
  slug: string
  builtin: boolean
  benchmark_count: number
  in_profiles: number
}
export interface GroupDetail {
  slug: string
  builtin: boolean
  benchmarks: GroupBenchmark[]
}

export interface BenchRow {
  model: string
  reasoning: string
  value: number
  norm: number
}
export interface BenchmarkDetail {
  name: string
  note: string
  groups: string[]
  rows: BenchRow[]
}

export interface ModelBenchRow {
  name: string
  value: number
  norm: number
  groups: string[]
}
export interface ModelScoreDetail {
  model: string
  reasoning: string
  rows: ModelBenchRow[]
}

/** One distinct catalog identity (scores CSV model name). */
export interface CatalogModel {
  model_name: string
  model_id: string
  reasoning: string[]
  intelligence: number | null
  cost: number | null
  speed: number | null
  provider_count: number
  maker?: string
  providers?: string[]
}

/** One enabled provider that serves a catalog model. Costs are USD / 1M tokens. */
export interface CatalogModelProvider {
  provider: string
  model_id: string
  reasoning: string[]
  route_keys: string[]
  input_cost_usd_per_m: number | null
  output_cost_usd_per_m: number | null
}

/** Model card: catalog identity plus enabled-provider rows. */
export interface CatalogModelDetail {
  model_name: string
  model_id: string
  reasoning: string[]
  intelligence: number | null
  cost: number | null
  speed: number | null
  provider_count: number
  in_catalog: boolean
  providers: CatalogModelProvider[]
}

export interface Favourite {
  route_key: string
  model_name: string
  route_label: string
  in_range: boolean
}

export interface GUISettings {
  user_profile: string
  layout: 'carousel' | 'list'
  /** Which popover tab opens by default. Shipped default: 'profiles'. */
  default_tab: 'profiles' | 'sliders'
  weight_control: 'step' | 'bar' | 'slider'
  holds: number
  shortcut: 'alt+space' | 'ctrl+space' | 'cmd+shift+m'
  show_menu_bar_icon: boolean
  launch_at_login: boolean
  copy_command_instead: boolean
  close_popover_after_launch: boolean
  auto_update: boolean
  auto_update_frequency: 'hourly' | 'daily' | 'weekly' | 'monthly'
  mcp_server: boolean
  claude_md_hint: boolean
  shell_alias: boolean
  /** Prefer the OS keychain for credentials created by which-model. */
  use_keychain: boolean
  /** GitHub owner/repo (optional @ref) to pull scores from. Default: WD-Mitchell/which-model. */
  catalog_repo: string
  /** When true, collect scores with a local Artificial Analysis API key instead of catalog_repo. */
  use_local_aa: boolean
  /** Restrict catalog display and recommendations to models from enabled providers. */
  only_enabled_providers: boolean
  allow_incomplete_recommendations: boolean
  /** How often to check for and sync new benchmark scores. Default: '6h'. */
  benchmark_check_frequency: '15m' | '1h' | '3h' | '6h' | '12h' | '24h' | 'weekly'
  /** Write-only. Get always returns "". Set a new key, or "-" to clear. */
  aa_api_key: string
  /** True when a local AA API key is stored. */
  aa_api_key_set: boolean
  config_path: string
  version: string
}

export interface ShellSnippets {
  alias: string
  claude_md: string
  preview: string
}

export interface ProfileStats {
  picks: number
  last_used: string
}

export interface ErrorDTO {
  code: string
  message: string
}

/** A curated default set of use cases for a kind of work. */
export interface UserProfile {
  slug: string
  name: string
  description: string
  use_case_slugs: string[]
  default_use_case: string
}
