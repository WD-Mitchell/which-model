//go:build !nousage

// Package claude is the Go port of usage-allowance-checks/lib/claude.mjs
// (91 lines): the Claude (Anthropic) subscription-usage adapter reporting the
// 5-hour and weekly rate-limit windows from GET
// https://api.anthropic.com/api/oauth/usage
// (specs/features/F15-provider-claude/SPEC.md).
package claude

import (
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// UsageURL is the exact allow-listed endpoint (claude.mjs:15).
const UsageURL = "https://api.anthropic.com/api/oauth/usage"

// UserAgent is the fixed client identity sent on the usage request
// (annex-a §3.2; CodexBar fallback version, survey:135).
const UserAgent = "claude-code/2.1.0"

// DefaultFilePaths are the ordered credential file candidates, dot-file
// first (annex-a §3.2 items 3-4). "$HOME" is expanded by the caller (F12).
const (
	DotFileRelativePath   = ".claude/.credentials.json"
	PlainFileRelativePath = ".claude/credentials.json"
)

// Error is the provider failure type. Code is always a value from
// specs/global/CONTRACTS.md §1.6; Message is a sanitized fixed string that
// never contains credential material.
type Error struct {
	Code    string
	Message string
}

// Error renders "<code>: <message>".
func (e *Error) Error() string { return e.Code + ": " + e.Message }

func init() {
	usage.Register(usage.Descriptor{
		ID:          "claude",
		DisplayName: "Claude",
		Kind:        usage.KindSubscription,
		Tier:        1,
		Auth: []usage.AuthSource{
			// 1. operator override (annex-a §3.2 item 1)
			{Kind: usage.AuthEnvVar, EnvVar: "WHICH_MODEL_CLAUDE_OAUTH_TOKEN"},
			// 2. macOS keychain (annex-a §3.2 item 2); F12 gates to darwin
			{Kind: usage.AuthKeychainGeneric, Keychain: &usage.KeychainSpec{Service: "Claude Code-credentials"}},
			// 3-8. files, dot-file first; one entry per tolerated token shape
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "claudeAiOauth.accessToken"},
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "claudeAiOauth.access_token"},
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "oauth.accessToken"},
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "oauth.access_token"},
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "accessToken"},
			{Kind: usage.AuthFile, FilePaths: []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}, JSONPath: "access_token"},
		},
		Windows: []usage.WindowSpec{
			{ID: "5h", Label: "five hour", Unit: usage.UnitPercent},                 // always present (real or synthetic, SPEC D5)
			{ID: "weekly", Label: "seven day", Unit: usage.UnitPercent, Optional: true},
			{ID: "sonnet_7d", Label: "seven day Sonnet", Unit: usage.UnitPercent, Optional: true},
			{ID: "opus_7d", Label: "seven day Opus", Unit: usage.UnitPercent, Optional: true},
			{ID: "oauth_apps_7d", Label: "seven day OAuth apps", Unit: usage.UnitPercent, Optional: true},
			{ID: "routines_7d", Label: "seven day Routines", Unit: usage.UnitPercent, Optional: true},
			{ID: "extra_usage", Label: "Extra usage", Unit: usage.UnitUSD, Optional: true},
		},
		Timeout:  15 * time.Second,
		CacheTTL: 60 * time.Second,
		Fetch:    Fetch,
	})
}
