//go:build !nousage

// Package codex is the Tier-1 Codex / ChatGPT subscription-usage adapter
// (specs/features/F16-provider-codex/SPEC.md). It is the Go port of
// usage-allowance-checks/lib/codex.mjs: it reports the primary/secondary
// rate-limit windows plus the credits balance from
// GET https://chatgpt.com/backend-api/wham/usage, with the prototype's
// fallback to a configured, explicitly trusted origin. It self-registers a
// usage.Descriptor with ID "codex".
package codex

import (
	"context"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// UsageURL is the exact allow-listed primary endpoint (codex.mjs:5).
const UsageURL = "https://chatgpt.com/backend-api/wham/usage"

// FallbackStatuses are the only statuses that may trigger the trusted-origin
// fallback (codex.mjs:6).
var FallbackStatuses = map[int]bool{404: true, 405: true, 410: true, 501: true}

// trustedOriginKey is the unexported context key for WithTrustedOrigin.
type trustedOriginKey struct{}

// WithTrustedOrigin stores the per-invocation trusted origin (F25's
// --trust-configured-origin) for Fetch. Absent value == untrusted.
func WithTrustedOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, trustedOriginKey{}, origin)
}

// TrustedOriginFrom extracts the value set by WithTrustedOrigin ("" when unset).
func TrustedOriginFrom(ctx context.Context) string {
	if v, ok := ctx.Value(trustedOriginKey{}).(string); ok {
		return v
	}
	return ""
}

// Error is the provider failure type. Code is always a value from
// specs/global/CONTRACTS.md §1.6; Message is a sanitized fixed string that
// never contains credential material.
type Error struct {
	Code    string
	Message string
}

// Error renders "<code>: <message>".
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Fetch is the FetchFunc port of checkCodexUsage (codex.mjs:102-117) per SPEC
// §2.5-§2.9, §2.12 (implemented in codex_fetch.go).

// init registers the descriptor (CONTRACTS §2). Duplicate IDs panic.
func init() {
	usage.Register(usage.Descriptor{
		ID:          "codex",
		DisplayName: "Codex",
		Kind:        usage.KindSubscription,
		Tier:        1,
		Auth: []usage.AuthSource{
			// one entry per tolerated token shape, over [$CODEX_HOME, ~/.codex]/auth.json
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "tokens.access_token"},
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "tokens.accessToken"},
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "auth.access_token"},
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "auth.accessToken"},
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "access_token"},
			{Kind: usage.AuthFile, FilePaths: []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}, JSONPath: "accessToken"},
		},
		Windows: []usage.WindowSpec{
			{ID: "5h", Label: "primary window", Unit: usage.UnitPercent, Optional: true},
			{ID: "weekly", Label: "secondary window", Unit: usage.UnitPercent, Optional: true},
			{ID: "credits", Label: "credits", Unit: usage.UnitCredits, Optional: true},
		},
		Timeout:  15 * time.Second,
		CacheTTL: 60 * time.Second,
		Fetch:    Fetch,
	})
}
