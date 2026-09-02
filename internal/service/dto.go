// Package service is the desktop app's Wails-free programmatic surface over
// the which-model engine (B00 SPEC). This file (B02, D00-owned) holds the
// canonical boundary DTOs plus the route-key parser/formatter; the TS mirrors
// live in packages/core/src/types.ts with identical names and snake_case JSON
// keys (D00 CONTRACTS §2).
package service

import (
	"fmt"
	"regexp"
	"strings"
)

// Route-key grammar (D00 CONTRACTS §1).
var (
	providerRe = regexp.MustCompile(`^[a-z0-9_]+$`)
	modelIDRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// reasoningLevels is the closed reasoning enum (D00 CONTRACTS §1).
var reasoningLevels = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":   true,
	"max":     true,
	"default": true,
}

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

// ProfileDetail is the same shape used for editing.
type ProfileDetail = ProfileSummary

// RankRequest.Overrides non-nil ⇒ ephemeral ranking: nothing is persisted and
// history is NOT written. Holds ∈ {3,5,10}; 0 means "use [gui].holds".
type RankRequest struct {
	ProfileSlug string         `json:"profile_slug"`
	Overrides   *ProfileDetail `json:"overrides,omitempty"`
	Holds       int            `json:"holds"`
}

// RankedModel is one ranked candidate.
type RankedModel struct {
	Rank      int     `json:"rank"` // 1-based
	ModelID   string  `json:"model_id"`
	ModelName string  `json:"model_name"`
	Provider  string  `json:"provider"`
	Reasoning string  `json:"reasoning"`
	Score     float64 `json:"score"` // already rounded to 2dp
	RouteKey  string  `json:"route_key"`
}

// RankResponse is a ranked result set.
type RankResponse struct {
	Candidates []RankedModel `json:"candidates"` // top Holds, rank ascending
	Total      int           `json:"total"`      // candidates before truncation
}

// CatalogSummary counts the live catalog.
type CatalogSummary struct {
	Models      int `json:"models"` // distinct (model, reasoning) rows in scores CSV
	ProvidersOn int `json:"providers_on"`
	Harnesses   int `json:"harnesses"`
}

// ProviderInfo is one provider's list/read shape.
type ProviderInfo struct {
	ID          string `json:"id"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`    // 1-based display order
	Auth        string `json:"auth"`        // e.g. "oauth", "device flow", "" when unknown
	LimitsLine  string `json:"limits_line"` // human summary or "not enabled"
	RoutesOn    int    `json:"routes_on"`
	RoutesTotal int    `json:"routes_total"`
	Session     *int   `json:"session"` // used %, nil when unknown
	Weekly      *int   `json:"weekly"`
	Monthly     *int   `json:"monthly"`
	Credits     string `json:"credits"`
	Resets      string `json:"resets"`
	Accounts    int    `json:"accounts"` // configured account count
	Builtin     bool   `json:"builtin"`  // ships a usage adapter -> not deletable
}

// RouteLevel is one reasoning level of a provider model.
type RouteLevel struct {
	Reasoning string `json:"reasoning"`
	Enabled   bool   `json:"enabled"`
	Default   bool   `json:"default"` // level == the model's top reasoning
}

// ProviderModel is one provider model with its reasoning levels.
type ProviderModel struct {
	ModelID   string       `json:"model_id"`
	ModelName string       `json:"model_name"`
	Levels    []RouteLevel `json:"levels"`
}

// AccountKind is the credential a provider account links to.
const (
	AccountKindOAuth  = "oauth"
	AccountKindCookie = "cookie"
	AccountKindToken  = "token"
)

// ProviderAccountDTO is one named credential for a provider. Ref is a
// REFERENCE (env var, file path, keychain service) — never the secret itself.
type ProviderAccountDTO struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // oauth | cookie | token
	Ref  string `json:"ref"`
}

// ProviderDetail is the provider edit/read shape.
type ProviderDetail struct {
	ID       string               `json:"id"`
	Models   []ProviderModel      `json:"models"`
	Accounts []ProviderAccountDTO `json:"accounts"`
	// Builtin providers ship a usage adapter and cannot be deleted; the UI
	// disables Delete for them rather than failing the call.
	Builtin bool `json:"builtin"`
}

// HarnessInfo is one harness's shape.
type HarnessInfo struct {
	Slug      string          `json:"slug"`
	Name      string          `json:"name"`
	Command   string          `json:"command"` // template with {model_id}/{reasoning}
	Builtin   bool            `json:"builtin"`
	Installed bool            `json:"installed"` // argv[0] found on PATH
	Providers map[string]bool `json:"providers"` // per-harness provider allow-map
}

// LaunchResult is the harness launch outcome.
type LaunchResult struct {
	Copied  bool   `json:"copied"`  // true ⇒ frontend puts Command on the clipboard
	Command string `json:"command"` // fully substituted
}

// UsageWindow is one usage window (session|weekly|monthly|provider-specific).
type UsageWindow struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	UsedPercent *int   `json:"used_percent"` // nil when unknown
	ResetHint   string `json:"reset_hint"`
	Unlimited   bool   `json:"unlimited"`
}

// UsageDTO is a provider's usage snapshot.
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

// GroupBenchmark is one benchmark's membership within a group detail.
type GroupBenchmark struct {
	Name          string `json:"name"`
	On            bool   `json:"on"`
	Covered       int    `json:"covered"` // (model,reasoning) rows reporting it
	CoverageTotal int    `json:"coverage_total"`
}

// GroupSummary is one group's list shape.
type GroupSummary struct {
	Slug           string `json:"slug"`
	Builtin        bool   `json:"builtin"`
	BenchmarkCount int    `json:"benchmark_count"`
	InProfiles     int    `json:"in_profiles"`
}

// GroupDetail is one group's edit/read shape.
type GroupDetail struct {
	Slug       string           `json:"slug"`
	Builtin    bool             `json:"builtin"`
	Benchmarks []GroupBenchmark `json:"benchmarks"` // full catalogue, On marks membership
}

// BenchRow is one tested benchmark row.
type BenchRow struct {
	Model     string  `json:"model"`
	Reasoning string  `json:"reasoning"`
	Value     float64 `json:"value"` // raw benchmark result
	Norm      float64 `json:"norm"`  // value / max * 100, rounded to integer-valued float
}

// BenchmarkDetail is one benchmark's detail shape.
type BenchmarkDetail struct {
	Name   string     `json:"name"`
	Note   string     `json:"note"`   // description; "" when none recorded
	Groups []string   `json:"groups"` // group slugs containing it
	Rows   []BenchRow `json:"rows"`   // tested rows only, Norm desc by default
}

// ModelBenchRow is one benchmark result for a (model, reasoning) pair.
type ModelBenchRow struct {
	Name   string   `json:"name"`
	Value  float64  `json:"value"`
	Norm   float64  `json:"norm"` // value / max * 100 across every tested model
	Groups []string `json:"groups"`
}

// ModelScoreDetail is the inverse of BenchmarkDetail: every benchmark
// (model, reasoning) reports. An unknown or untested pair returns empty Rows,
// not not_found — Settings always lets you open a catalogue combo.
type ModelScoreDetail struct {
	Model     string          `json:"model"`
	Reasoning string          `json:"reasoning"`
	Rows      []ModelBenchRow `json:"rows"`
}

// Favourite is one pinned route.

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

// CatalogModelProvider is one enabled provider that serves a catalog model.
// Costs are USD per 1M tokens from the models.dev cache; nil means no listed price.
type CatalogModelProvider struct {
	Provider          string   `json:"provider"`
	ModelID           string   `json:"model_id"`
	Reasoning         []string `json:"reasoning"`
	RouteKeys         []string `json:"route_keys"`
	InputCostUSDPerM  *float64 `json:"input_cost_usd_per_m"`
	OutputCostUSDPerM *float64 `json:"output_cost_usd_per_m"`
}

// CatalogModelDetail is the model card: catalog identity plus enabled-provider rows.
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

// GUISettings is the settings-page aggregate for [gui] plus [auth].use_keychain.
type GUISettings struct {
	Layout                  string `json:"layout"`         // "carousel"|"list"
	DefaultTab              string `json:"default_tab"`    // "profiles"|"sliders"
	WeightControl           string `json:"weight_control"` // "step"|"bar"|"slider"
	Holds                   int    `json:"holds"`          // 3|5|10
	Shortcut                string `json:"shortcut"`       // "alt+space"|"ctrl+space"|"cmd+shift+m"
	ShowMenuBarIcon         bool   `json:"show_menu_bar_icon"`
	LaunchAtLogin           bool   `json:"launch_at_login"`
	CopyCommandInstead      bool   `json:"copy_command_instead"`
	ClosePopoverAfterLaunch bool   `json:"close_popover_after_launch"`
	AutoUpdate              bool   `json:"auto_update"`
	AutoUpdateFrequency     string `json:"auto_update_frequency"` // "hourly"|"daily"|"weekly"|"monthly"
	MCPServer               bool   `json:"mcp_server"`
	ClaudeMDHint            bool   `json:"claude_md_hint"`
	ShellAlias              bool   `json:"shell_alias"`
	UseKeychain             bool   `json:"use_keychain"` // [auth], true prefers OS keychain
	CatalogRepo             string `json:"catalog_repo"` // "owner/repo" or "owner/repo@ref"
	UseLocalAA              bool   `json:"use_local_aa"`
	OnlyEnabledProviders    bool   `json:"only_enabled_providers"`
	AAAPIKey                string `json:"aa_api_key"` // write-only; Get always returns ""
	AAAPIKeySet             bool   `json:"aa_api_key_set"`
	ConfigPath              string `json:"config_path"` // read-only, display only
	Version                 string `json:"version"`     // read-only, display only
}

// ShellSnippets are the copyable setup snippets.
type ShellSnippets struct {
	Alias    string `json:"alias"`
	ClaudeMD string `json:"claude_md"`
	Preview  string `json:"preview"` // "$ wm <slug>  →  <model_id>  (<provider>)" line
}

// ProfileStats is the D00 canon per-profile pick stats DTO.
type ProfileStats struct {
	Picks    int    `json:"picks"`
	LastUsed string `json:"last_used"` // RFC3339 or ""
}

// ErrorDTO is the boundary error shape; it implements error so hosts return
// it directly.
type ErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ParseRouteKey splits s at the FIRST '/' and the LAST '@', then validates
// each component. Grammar (D00 CONTRACTS §1):
//
//	route_key = provider "/" model_id "@" reasoning
//
// Every failure wraps errValidation (SPEC §2.8) with the exact messages in
// B02 CONTRACTS §6.
func ParseRouteKey(s string) (provider, modelID, reasoning string, err error) {
	slash := strings.IndexByte(s, '/')
	if slash < 0 {
		return "", "", "", fmt.Errorf("%w: route key %q: missing \"/\"", errValidation, s)
	}
	provider = s[:slash]
	rest := s[slash+1:]
	at := strings.LastIndexByte(rest, '@')
	if at < 0 {
		return "", "", "", fmt.Errorf("%w: route key %q: missing \"@\"", errValidation, s)
	}
	modelID = rest[:at]
	reasoning = rest[at+1:]

	if provider == "" {
		return "", "", "", fmt.Errorf("%w: route key %q: empty provider", errValidation, s)
	}
	if modelID == "" {
		return "", "", "", fmt.Errorf("%w: route key %q: empty model_id", errValidation, s)
	}
	if reasoning == "" {
		return "", "", "", fmt.Errorf("%w: route key %q: empty reasoning", errValidation, s)
	}
	if !providerRe.MatchString(provider) {
		return "", "", "", fmt.Errorf("%w: route key %q: invalid provider %q", errValidation, s, provider)
	}
	if !modelIDRe.MatchString(modelID) {
		return "", "", "", fmt.Errorf("%w: route key %q: invalid model_id %q", errValidation, s, modelID)
	}
	if !reasoningLevels[reasoning] {
		return "", "", "", fmt.Errorf("%w: route key %q: invalid reasoning %q", errValidation, s, reasoning)
	}
	return provider, modelID, reasoning, nil
}

// FormatRouteKey returns provider + "/" + modelID + "@" + reasoning. No
// validation (SPEC §2.8): callers hold already-valid parts.
func FormatRouteKey(provider, modelID, reasoning string) string {
	return provider + "/" + modelID + "@" + reasoning
}
