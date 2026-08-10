// Package usage defines the canonical usage types and the provider
// descriptor registry (specs/features/F11-usage-types/SPEC.md).
package usage

import "time"

// Unit is the dimension of a Window's readings (specs/global/CONTRACTS.md §1.1).
type Unit string

const (
	UnitPercent   Unit = "percent"
	UnitTokens    Unit = "tokens"
	UnitCredits   Unit = "credits"
	UnitUSD       Unit = "usd"
	UnitRequests  Unit = "requests"
	UnitEnergyKWh Unit = "kwh"
	UnitNone      Unit = "none"
)

// Source records where a Snapshot came from (specs/global/CONTRACTS.md §1.2).
type Source string

const (
	SourceOAuth Source = "oauth"
	SourceAPI   Source = "api"
	SourceCLI   Source = "cli"
	SourceWeb   Source = "web"
	SourceLocal Source = "local"
	SourceCache Source = "cache"
)

// Kind classifies a provider's billing model (specs/global/CONTRACTS.md §1.3).
type Kind int

const (
	KindSubscription  Kind = iota // Claude, Codex, Copilot, Cursor, etc.
	KindAPIKeyBilling             // OpenAI platform, DeepInfra, etc.
	KindGateway                   // OpenRouter, LiteLLM, ClawRouter, etc.
	KindLocalTool                 // Ollama, JetBrains (presence-only)
)

// String renders the Kind for humans ("subscription", "api_key_billing", "gateway", "local_tool", "unknown").
func (k Kind) String() string {
	switch k {
	case KindSubscription:
		return "subscription"
	case KindAPIKeyBilling:
		return "api_key_billing"
	case KindGateway:
		return "gateway"
	case KindLocalTool:
		return "local_tool"
	default:
		return "unknown"
	}
}

// Window is one quota/usage window reading (specs/global/CONTRACTS.md §1.4).
type Window struct {
	ID            string     `json:"id"`
	Label         string     `json:"label"`
	Unit          Unit       `json:"unit"`
	UsedPercent   *float64   `json:"used_percent,omitempty"`
	Used          *float64   `json:"used,omitempty"`
	Limit         *float64   `json:"limit,omitempty"`
	Remaining     *float64   `json:"remaining,omitempty"`
	Unlimited     bool       `json:"unlimited,omitempty"`
	WindowMinutes *int       `json:"window_minutes,omitempty"`
	ResetsAt      *time.Time `json:"resets_at,omitempty"`
	ResetHint     string     `json:"reset_hint,omitempty"`
	ModelScope    []string   `json:"model_scope,omitempty"`
	Synthetic     bool       `json:"synthetic,omitempty"`
	UsageKnown    bool       `json:"usage_known"`
}

// Snapshot is one provider's usage reading (specs/global/CONTRACTS.md §1.5).
type Snapshot struct {
	Provider   string    `json:"provider"`
	Account    string    `json:"account,omitempty"`
	Plan       string    `json:"plan,omitempty"`
	Windows    []Window  `json:"windows"`
	FetchedAt  time.Time `json:"fetched_at"`
	Source     Source    `json:"source"`
	Confidence string    `json:"confidence"`  // "live" | "cached" | "estimated"
	UsageKnown bool      `json:"usage_known"` // at least one window carries a real reading (see DEFERRED D5)
	Stale      bool      `json:"stale,omitempty"`
	Failure    *Failure  `json:"error,omitempty"`
}

// Failure is a stable error code + sanitised message
// (specs/global/CONTRACTS.md §1.6). Message NEVER contains credential material.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"` // sanitised; NEVER contains credential material
}

// Warning is a non-fatal, sanitised diagnostic returned alongside usage
// snapshots.
type Warning struct {
	Message string
}

// WindowSpec is descriptor-time metadata: which window IDs/labels/units a
// provider MAY report, and which models each window's quota applies to.
// It is NOT a runtime reading — that is Window (global CONTRACTS §1.4).
// Tag-free (moved from descriptor.go, DEFERRED D13): pure data, no
// credential surface, needed by the catalog-side routing join (F18) under
// -tags nousage.
type WindowSpec struct {
	ID         string
	Label      string
	Unit       Unit
	Optional   bool     // provider may omit this window depending on plan/quota shape
	ModelScope []string // model IDs this window's quota applies to (F18 BindWindowIDs, annex-b §7.3)
}
