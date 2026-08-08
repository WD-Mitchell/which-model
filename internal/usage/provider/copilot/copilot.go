//go:build !nousage

// Package copilot is the Tier-1 GitHub Copilot subscription-usage adapter:
// the Go port of usage-allowance-checks/lib/copilot.mjs (285 lines),
// normalized into the canonical usage.Window contract. It reports the
// premium-interactions, chat, and completions quota windows from the private
// endpoint GET https://api.github.com/copilot_internal/user, always behind the
// mandatory identity gate (GET https://api.github.com/user), and self-registers
// a usage.Descriptor with ID "copilot". The device-flow state machine is
// exported for F12's declarative AuthOAuthDeviceFlow resolver and F25's
// interactive --login path (specs/features/F17-provider-copilot/SPEC.md).
package copilot

import (
	"context"
	"net/http"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Endpoint and OAuth constants (verbatim copilot.mjs:9-15).
const (
	GitHubDeviceCodeURL  = "https://github.com/login/device/code"
	GitHubDeviceTokenURL = "https://github.com/login/oauth/access_token"
	GitHubUserURL        = "https://api.github.com/user"
	CopilotUsageURL      = "https://api.github.com/copilot_internal/user"
	CopilotClientID      = "Iv1.b507a08c87ecfe98"
	APIVersion           = "2025-04-01"
)

// IdentityUserAgent is the identity-gate User-Agent (annex-a §3.3; SPEC D4).
const IdentityUserAgent = "which-model/0.4.0"

// validateIdentityHook adapts ValidateIdentity to the canonical
// AuthSource.Validate shape (F11 internal/usage/descriptor.go):
// func(ctx, usage.Credential, *http.Client) error. The identity gate runs on
// EVERY chain candidate; a candidate failing it is skipped and the chain
// continues (F12 chain rule, SPEC §2.1).
func validateIdentityHook(ctx context.Context, cred usage.Credential, client *http.Client) error {
	_, err := ValidateIdentity(ctx, cred.Token, client)
	return err
}

// init registers the copilot descriptor. The literal matches CONTRACTS §2
// field-for-field except where the canonical F11 types differ (per F17-T1
// instruction 3): ShellSpec carries no MaxOutputBytes (the 32 KiB CLI output
// cap is F12's CLIResolver constant, SPEC D2) and AuthSource.Validate takes a
// usage.Credential (see validateIdentityHook).
func init() {
	usage.Register(usage.Descriptor{
		ID:          "copilot",
		DisplayName: "GitHub Copilot",
		Kind:        usage.KindSubscription,
		Tier:        1,
		Auth: []usage.AuthSource{
			// 1. operator override
			{Kind: usage.AuthEnvVar, EnvVar: "COPILOT_API_TOKEN", Validate: validateIdentityHook},
			// 2-4. prototype discovery order (copilot.mjs:48-52); never --local
			{Kind: usage.AuthCLIShellOut, Shell: &usage.ShellSpec{Command: "git", Args: []string{"config", "--global", "--get", "github.copilot.oauthToken"}, Timeout: 3 * time.Second}, Validate: validateIdentityHook},
			{Kind: usage.AuthCLIShellOut, Shell: &usage.ShellSpec{Command: "git", Args: []string{"config", "--system", "--get", "github.copilot.oauthToken"}, Timeout: 3 * time.Second}, Validate: validateIdentityHook},
			{Kind: usage.AuthCLIShellOut, Shell: &usage.ShellSpec{Command: "gh", Args: []string{"auth", "token", "--hostname", "github.com"}, Timeout: 3 * time.Second}, Validate: validateIdentityHook},
			// 5. --login path only (F25); F12 MUST delegate to StartDeviceFlow/PollDeviceFlow
			{Kind: usage.AuthOAuthDeviceFlow, OAuth: &usage.OAuthSpec{ClientID: CopilotClientID, DeviceCodeURL: GitHubDeviceCodeURL, TokenURL: GitHubDeviceTokenURL, Scope: "read:user"}, Validate: validateIdentityHook},
		},
		Windows: []usage.WindowSpec{
			{ID: "premium", Label: "premium interactions", Unit: usage.UnitRequests, Optional: true},
			{ID: "chat", Label: "chat", Unit: usage.UnitRequests, Optional: true},
			{ID: "completions", Label: "completions", Unit: usage.UnitRequests, Optional: true},
		},
		Timeout:  15 * time.Second,
		CacheTTL: 60 * time.Second,
		Fetch:    Fetch,
	})
}
