---
kind: feature-contracts
version: "1.0"
feature: D00-global
project: which-model-desktop
---

# D00-global — Contracts

These types are THE canonical DTO definitions. Feature contracts reference them by name and MUST NOT redefine them. Go structs live in `internal/service/dto.go`; the TS mirrors live in `packages/core/src/types.ts` with identical names and `snake_case` JSON keys.

## 1. Route key grammar

```
route_key   = provider "/" model_id "@" reasoning
provider    = [a-z0-9_]+            ; e.g. "claude", "codex", "copilot", "cursor"
model_id    = [A-Za-z0-9._-]+       ; engine model id, e.g. "claude-opus-5"
reasoning   = "minimal"|"low"|"medium"|"high"|"xhigh"|"max"|"default"
```
This is the ONLY serialised route identity (favourites pins, `[routes.disabled]` values, RankedModel.RouteKey). Parser/formatter: `service.ParseRouteKey(s string) (provider, modelID, reasoning string, err error)` / `service.FormatRouteKey(provider, modelID, reasoning string) string`. Invalid grammar anywhere → `validation_failed`.

## 2. Canonical DTOs (Go; TS mirrors field-for-field)

```go
package service

// ProfileSummary describes one profile for lists; ProfileDetail is the same
// shape used for editing. Weights are ints 0–5 where 0 means "absent/ignored"
// (the TOML stores only weights 1–5; 0-valued keys are removed on save).
// CoreShare is the tier-1 share as an integer percent, clamped 10..90, step 5.
type ProfileSummary struct {
    Slug         string         `json:"slug"`
    Name         string         `json:"name"`
    Builtin      bool           `json:"builtin"`
    CoreShare    int            `json:"core_share"`
    Tier1Weights map[string]int `json:"tier1_weights"` // keys ⊆ {intelligence, cost, speed}
    Tier2Weights map[string]int `json:"tier2_weights"` // keys ⊆ categories ∪ custom group slugs
    Picks        int            `json:"picks"`
    LastUsed     string         `json:"last_used"` // RFC3339 or ""
}
type ProfileDetail = ProfileSummary

// RankRequest.Overrides non-nil ⇒ ephemeral ranking: nothing is persisted and
// history is NOT written. Holds ∈ {3,5,10}; 0 means "use [gui].holds".
type RankRequest struct {
    ProfileSlug string         `json:"profile_slug"`
    Overrides   *ProfileDetail `json:"overrides,omitempty"`
    Holds       int            `json:"holds"`
}

type RankedModel struct {
    Rank      int     `json:"rank"`       // 1-based
    ModelID   string  `json:"model_id"`
    ModelName string  `json:"model_name"`
    Provider  string  `json:"provider"`
    Reasoning string  `json:"reasoning"`
    Score     float64 `json:"score"`      // already rounded to 2dp
    RouteKey  string  `json:"route_key"`
}

type RankResponse struct {
    Candidates []RankedModel `json:"candidates"` // top Holds, rank ascending
    Total      int           `json:"total"`      // candidates before truncation
}

type CatalogSummary struct {
    Models      int `json:"models"`       // distinct (model, reasoning) rows in scores CSV
    ProvidersOn int `json:"providers_on"`
    Harnesses   int `json:"harnesses"`
}

type ProviderInfo struct {
    ID          string `json:"id"`
    Enabled     bool   `json:"enabled"`
    Priority    int    `json:"priority"`   // 1-based display order
    Auth        string `json:"auth"`       // e.g. "oauth", "device flow", "" when unknown
    LimitsLine  string `json:"limits_line"`// human summary or "not enabled"
    RoutesOn    int    `json:"routes_on"`
    RoutesTotal int    `json:"routes_total"`
    Session     *int   `json:"session"`    // used %, nil when unknown
    Weekly      *int   `json:"weekly"`
    Monthly     *int   `json:"monthly"`
    Credits     string `json:"credits"`
    Resets      string `json:"resets"`
}

type RouteLevel struct {
    Reasoning string `json:"reasoning"`
    Enabled   bool   `json:"enabled"`
    Default   bool   `json:"default"` // level == the model's top reasoning
}
type ProviderModel struct {
    ModelID   string       `json:"model_id"`
    ModelName string       `json:"model_name"`
    Levels    []RouteLevel `json:"levels"`
}
type ProviderDetail struct {
    ID     string          `json:"id"`
    Models []ProviderModel `json:"models"`
}

type HarnessInfo struct {
    Slug      string          `json:"slug"`
    Name      string          `json:"name"`
    Command   string          `json:"command"`   // template with {model_id}/{reasoning}
    Builtin   bool            `json:"builtin"`
    Installed bool            `json:"installed"` // argv[0] found on PATH
    Providers map[string]bool `json:"providers"` // per-harness provider allow-map
}

type LaunchResult struct {
    Copied  bool   `json:"copied"`  // true ⇒ frontend puts Command on the clipboard
    Command string `json:"command"` // fully substituted
}

type UsageWindow struct {
    ID          string `json:"id"`           // "session"|"weekly"|"monthly"|provider-specific
    Label       string `json:"label"`
    UsedPercent *int   `json:"used_percent"` // nil when unknown
    ResetHint   string `json:"reset_hint"`
    Unlimited   bool   `json:"unlimited"`
}
type UsageDTO struct {
    Provider   string        `json:"provider"`
    Plan       string        `json:"plan"`
    Auth       string        `json:"auth"`
    Confidence string        `json:"confidence"` // "live"|"cached"|"estimated"
    Stale      bool          `json:"stale"`
    Windows    []UsageWindow `json:"windows"`
    Credits    string        `json:"credits"`
    Resets     string        `json:"resets"`
    Failure    string        `json:"failure"` // "" when ok; sanitised message otherwise
}

type GroupBenchmark struct {
    Name          string `json:"name"`
    On            bool   `json:"on"`
    Covered       int    `json:"covered"`        // (model,reasoning) rows reporting it
    CoverageTotal int    `json:"coverage_total"`
}
type GroupSummary struct {
    Slug           string `json:"slug"`
    Builtin        bool   `json:"builtin"`
    BenchmarkCount int    `json:"benchmark_count"`
    InProfiles     int    `json:"in_profiles"`
}
type GroupDetail struct {
    Slug       string           `json:"slug"`
    Builtin    bool             `json:"builtin"`
    Benchmarks []GroupBenchmark `json:"benchmarks"` // full catalogue, On marks membership
}

type BenchRow struct {
    Model     string  `json:"model"`
    Reasoning string  `json:"reasoning"`
    Value     float64 `json:"value"` // raw benchmark result
    Norm      float64 `json:"norm"`  // value / max * 100, rounded to integer-valued float
}
type BenchmarkDetail struct {
    Name   string     `json:"name"`
    Note   string     `json:"note"`   // description; "" when none recorded
    Groups []string   `json:"groups"` // group slugs containing it
    Rows   []BenchRow `json:"rows"`   // tested rows only, Norm desc by default
}
type ModelBenchRow struct {
    Name   string   `json:"name"`
    Value  float64  `json:"value"`
    Norm   float64  `json:"norm"`
    Groups []string `json:"groups"`
}
type ModelScoreDetail struct {
    Model     string          `json:"model"`
    Reasoning string          `json:"reasoning"`
    Rows      []ModelBenchRow `json:"rows"`
}


// CatalogModel is one distinct catalog identity (scores CSV model name),
// aggregated across reasoning rows. Intelligence/Cost/Speed are the top
// reasoning level's tier-1 scores (0–100), nil when that axis is blank.
type CatalogModel struct {
    ModelName     string   `json:"model_name"`
    ModelID       string   `json:"model_id"` // representative route id; "" if none
    Reasoning     []string `json:"reasoning"`
    Intelligence  *float64 `json:"intelligence"`
    Cost          *float64 `json:"cost"`
    Speed         *float64 `json:"speed"`
    ProviderCount int      `json:"provider_count"`
}

// CatalogModelProvider is one enabled provider offering a catalog model, with
// reasoning levels, route keys, and models.dev pricing when listed (B05 §2.15).
type CatalogModelProvider struct {
    Provider          string   `json:"provider"`
    ModelID           string   `json:"model_id"`
    Reasoning         []string `json:"reasoning"`
    RouteKeys         []string `json:"route_keys"`
    InputCostUSDPerM  *float64 `json:"input_cost_usd_per_m"`
    OutputCostUSDPerM *float64 `json:"output_cost_usd_per_m"`
}

// CatalogModelDetail is the full model card: identity, reasoning, scores,
// and the list of enabled providers serving it.
type CatalogModelDetail struct {
    ModelName     string                 `json:"model_name"`
    ModelID       string                 `json:"model_id"`
    Reasoning     []string               `json:"reasoning"`
    Intelligence  *float64               `json:"intelligence"`
    Cost          *float64               `json:"cost"`
    Speed         *float64               `json:"speed"`
    ProviderCount int                    `json:"provider_count"`
    Providers     []CatalogModelProvider `json:"providers"`
}

type Favourite struct {
    RouteKey   string `json:"route_key"`
    ModelName  string `json:"model_name"`
    RouteLabel string `json:"route_label"` // "provider · reasoning" or "no provider · reasoning"
    InRange    bool   `json:"in_range"`    // route still resolvable
}

type GUISettings struct {
    Layout                  string `json:"layout"`                    // "carousel"|"list"
    WeightControl           string `json:"weight_control"`            // "step"|"bar"|"slider"
    Holds                   int    `json:"holds"`                     // 3|5|10
    Shortcut                string `json:"shortcut"`                  // "alt+space"|"ctrl+space"|"cmd+shift+m"
    ShowMenuBarIcon         bool   `json:"show_menu_bar_icon"`
    LaunchAtLogin           bool   `json:"launch_at_login"`
    CopyCommandInstead      bool   `json:"copy_command_instead"`
    ClosePopoverAfterLaunch bool   `json:"close_popover_after_launch"`
    AutoUpdate              bool   `json:"auto_update"`
    AutoUpdateFrequency     string `json:"auto_update_frequency"`     // "hourly"|"daily"|"weekly"|"monthly"
    MCPServer               bool   `json:"mcp_server"`
    ClaudeMDHint            bool   `json:"claude_md_hint"`
    ShellAlias              bool   `json:"shell_alias"`
    UseKeychain             bool   `json:"use_keychain"` // [auth]; default true
    CatalogRepo             string `json:"catalog_repo"` // "owner/repo" or "owner/repo@ref"
    UseLocalAA              bool   `json:"use_local_aa"`
    AAAPIKey                string `json:"aa_api_key"`   // write-only; Get returns ""
    AAAPIKeySet             bool   `json:"aa_api_key_set"`
    ConfigPath              string `json:"config_path"` // read-only, display only
}

type ShellSnippets struct {
    Alias    string `json:"alias"`
    ClaudeMD string `json:"claude_md"`
    Preview  string `json:"preview"` // "$ wm <slug>  →  <model_id>  (<provider>)" line
}

type ProfileStats struct {
    Picks    int    `json:"picks"`
    LastUsed string `json:"last_used"` // RFC3339 or ""
}

type ErrorDTO struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

## 3. Events (closed enum)

Go consts in `internal/service/events.go`; TS union in `packages/core/src/events.ts`.

| Event | Emitted by | Payload |
|---|---|---|
| `config:changed` | any config-mutating method except settings | `{"section": string}` |
| `catalog:changed` | group save/delete triggering re-derive | `{}` |
| `usage:updated` | usage refresher / forced fetch | `{}` |
| `settings:changed` | SettingsService.Set | `GUISettings` |
| `pick:recorded` | RecordPick / Launch | `{"profile_slug": string, "route_key": string}` |

## 4. Error codes (closed enum)

| Code | Meaning |
|---|---|
| `validation_failed` | input rejected (bad slug, bad enum, bad route key, unknown template token, weight out of range) |
| `builtin_readonly` | attempt to modify/delete a built-in profile, group, or harness |
| `not_found` | slug/id does not exist |
| `conflict` | slug already exists |
| `io_error` | file read/write failure; message names the path |
| `usage_unavailable` | usage disabled by config/backend or fetch impossible |
| `launch_failed` | harness process could not be spawned |

## 5. EngineHost (TS, `packages/core/src/host.ts`)

```ts
export interface EngineHost {
  profiles: {
    list(): Promise<ProfileSummary[]>
    get(slug: string): Promise<ProfileDetail>
    save(p: ProfileDetail): Promise<void>
    duplicate(slug: string): Promise<ProfileDetail>
    delete(slug: string): Promise<void>
    complexityScale(): Promise<string[]>
  }
  pick: {
    rank(req: RankRequest): Promise<RankResponse>
    recordPick(profileSlug: string, routeKey: string): Promise<void>
    catalogLine(): Promise<CatalogSummary>
  }
  catalog: {
    benchmarks(): Promise<string[]>
    benchmarkDetail(name: string): Promise<BenchmarkDetail>
    modelDetail(model: string, reasoning: string): Promise<ModelScoreDetail>
    models(): Promise<CatalogModel[]>
    model(name: string): Promise<CatalogModelDetail>
    groups(): Promise<GroupSummary[]>
    groupDetail(slug: string): Promise<GroupDetail>
    saveGroup(slug: string, benchmarks: string[], renameTo?: string): Promise<void>
    duplicateGroup(slug: string): Promise<GroupDetail>
    deleteGroup(slug: string): Promise<void>
  }
  providers: {
    list(): Promise<ProviderInfo[]>
    setEnabled(id: string, on: boolean): Promise<void>
    reorder(orderedIds: string[]): Promise<void>
    detail(id: string): Promise<ProviderDetail>
    setRouteEnabled(id: string, modelId: string, reasoning: string, on: boolean): Promise<void>
    setAllRoutes(id: string, on: boolean): Promise<void>
  }
  harnesses: {
    list(): Promise<HarnessInfo[]>
    save(h: HarnessInfo): Promise<void>
    delete(slug: string): Promise<void>
    setProvider(slug: string, provider: string, on: boolean): Promise<void>
    setAllProviders(slug: string, on: boolean): Promise<void>
    launch(slug: string, routeKey: string, profileSlug: string): Promise<LaunchResult>
  }
  usage: {
    snapshots(force: boolean): Promise<UsageDTO[]>
    setMode(mode: 'auto' | 'on' | 'off'): Promise<void>
    setBackend(backend: 'off' | 'native' | 'codexbar'): Promise<void>
    mode(): Promise<{ mode: string; backend: string }>
  }
  favourites: {
    list(): Promise<Favourite[]>
    pin(routeKey: string): Promise<void>
    unpin(routeKey: string): Promise<void>
  }
  settings: {
    get(): Promise<GUISettings>
    set(s: GUISettings): Promise<void>
    shellSnippets(): Promise<ShellSnippets>
  }
  window: {
    openSettings(): Promise<void>
    closeSettings(): Promise<void>
    hidePopover(): Promise<void>
    quit(): Promise<void>
    copyToClipboard(text: string): Promise<void>
  }
  on(event: EngineEvent, cb: (payload: unknown) => void): () => void
}
```
Rejected promises carry `ErrorDTO`. `MockEngineHost` (`packages/core/src/mock.ts`) implements the full interface with in-memory fixture data; `window.*` methods no-op.

## 6. Shared visual tokens (from the mockup; UI features cite, don't restate)

| Token | Value |
|---|---|
| Popover width | 400px; arrow 8px; corner radius 12px |
| Settings window | 820×520 content + titlebar; sidebar 184px |
| Toggle `.sw` | 30×17, radius 9, knob 13×13 at top 2/left 2, on-left 15px |
| Slider knob | 12×12 circle, bg `--color-bg`, ring `0 0 0 1.5px var(--color-accent)` |
| Weight scale | integers 0–5 (0 = removed) |
| Balance | 10..90, step 5 |
| Complexity scale | 5 stops at 0/25/50/75/100% |
| Score format | 2 decimal places; percent as integer |
| Mono stack | `ui-monospace, SFMono-Regular, Menlo, monospace` + tabular-nums |
| Toast | bottom-center, 34px up, 2.6s, `toastIn` 0.16s ease-out |

## 7. Files owned by D00

| File | Contents |
|---|---|
| `internal/service/dto.go` | every §2 struct, `ParseRouteKey`, `FormatRouteKey` |
| `internal/service/events.go` | §3 event consts |
| `packages/core/src/types.ts` | §2 TS mirrors |
| `packages/core/src/events.ts` | §3 TS union |
| `packages/core/src/host.ts` | §5 interface |

(Implementation of these files belongs to IM-B02 (Go) and IM-U01 (TS); ownership here means their SHAPE may only change by editing this contract.)
