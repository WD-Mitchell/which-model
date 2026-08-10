//go:build !nousage

package claude

import (
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// TestDescriptorRegistered asserts the package's init() registered the
// "claude" descriptor in the F11 registry (registry from F11,
// internal/usage/registry.go). F11's canonical lookup surface is usage.Get —
// the bool-returning Lookup is deliberately NOT exported (F11 SPEC D2).
func TestDescriptorRegistered(t *testing.T) {
	d, err := usage.Get("claude")
	if err != nil {
		t.Fatalf("usage.Get(%q) failed after package import: %v", "claude", err)
	}
	if d.ID != "claude" {
		t.Errorf("descriptor ID = %q, want %q", d.ID, "claude")
	}
	if d.DisplayName != "Claude" {
		t.Errorf("DisplayName = %q, want %q", d.DisplayName, "Claude")
	}
	if d.Kind != usage.KindSubscription {
		t.Errorf("Kind = %v, want %v", d.Kind, usage.KindSubscription)
	}
	if d.Tier != 1 {
		t.Errorf("Tier = %d, want 1", d.Tier)
	}
	if d.Timeout != 15*time.Second {
		t.Errorf("Timeout = %v, want 15s", d.Timeout)
	}
	if d.CacheTTL != 60*time.Second {
		t.Errorf("CacheTTL = %v, want 60s", d.CacheTTL)
	}

	// Windows in order, per CONTRACTS §2.
	wantIDs := []string{"5h", "weekly", "sonnet_7d", "opus_7d", "oauth_apps_7d", "routines_7d", "extra_usage"}
	if len(d.Windows) != len(wantIDs) {
		t.Fatalf("Windows = %d entries, want %d", len(d.Windows), len(wantIDs))
	}
	for i, want := range wantIDs {
		if d.Windows[i].ID != want {
			t.Errorf("Windows[%d].ID = %q, want %q", i, d.Windows[i].ID, want)
		}
	}
	if d.Windows[0].Optional {
		t.Errorf("window %q Optional = true, want false (always present, real or synthetic)", d.Windows[0].ID)
	}
	if d.Windows[0].Unit != usage.UnitPercent {
		t.Errorf("window %q Unit = %q, want %q", d.Windows[0].ID, d.Windows[0].Unit, usage.UnitPercent)
	}
	for i := 1; i < len(d.Windows); i++ {
		if !d.Windows[i].Optional {
			t.Errorf("window %q Optional = false, want true", d.Windows[i].ID)
		}
	}
	if d.Windows[6].Unit != usage.UnitUSD {
		t.Errorf("window %q Unit = %q, want %q", d.Windows[6].ID, d.Windows[6].Unit, usage.UnitUSD)
	}

	// Auth chain kinds in order (CONTRACTS §2).
	wantKinds := []usage.AuthKind{
		usage.AuthEnvVar,
		usage.AuthKeychainGeneric,
		usage.AuthFile, usage.AuthFile, usage.AuthFile, usage.AuthFile, usage.AuthFile, usage.AuthFile,
	}
	if len(d.Auth) != len(wantKinds) {
		t.Fatalf("Auth = %d entries, want %d", len(d.Auth), len(wantKinds))
	}
	for i, want := range wantKinds {
		if d.Auth[i].Kind != want {
			t.Errorf("Auth[%d].Kind = %v, want %v", i, d.Auth[i].Kind, want)
		}
	}

	// Env + keychain entries.
	if d.Auth[0].EnvVar != "WHICH_MODEL_CLAUDE_OAUTH_TOKEN" {
		t.Errorf("Auth[0].EnvVar = %q, want %q", d.Auth[0].EnvVar, "WHICH_MODEL_CLAUDE_OAUTH_TOKEN")
	}
	if d.Auth[1].Keychain == nil || d.Auth[1].Keychain.Service != "Claude Code-credentials" {
		t.Errorf("Auth[1].Keychain.Service = %+v, want %q", d.Auth[1].Keychain, "Claude Code-credentials")
	}

	// File entries: dot-file first, one JSONPath per tolerated shape (SPEC D1).
	wantPaths := []string{"~/.claude/.credentials.json", "~/.claude/credentials.json"}
	wantJSONPaths := []string{
		"claudeAiOauth.accessToken",
		"claudeAiOauth.access_token",
		"oauth.accessToken",
		"oauth.access_token",
		"accessToken",
		"access_token",
	}
	for i, wantPath := range wantJSONPaths {
		e := d.Auth[2+i]
		if len(e.FilePaths) != 2 || e.FilePaths[0] != wantPaths[0] || e.FilePaths[1] != wantPaths[1] {
			t.Errorf("Auth[%d].FilePaths = %v, want %v", 2+i, e.FilePaths, wantPaths)
		}
		if e.JSONPath != wantPath {
			t.Errorf("Auth[%d].JSONPath = %q, want %q", 2+i, e.JSONPath, wantPath)
		}
	}
}

// TestUsageURL pins the exact allow-listed endpoint (claude.mjs:15).
func TestUsageURL(t *testing.T) {
	if UsageURL != "https://api.anthropic.com/api/oauth/usage" {
		t.Errorf("UsageURL = %q, want %q", UsageURL, "https://api.anthropic.com/api/oauth/usage")
	}
	if UserAgent != "claude-code/2.1.0" {
		t.Errorf("UserAgent = %q, want %q", UserAgent, "claude-code/2.1.0")
	}
}
