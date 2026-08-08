//go:build !nousage

package copilot

import (
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// TestDescriptorRegistration covers F17-T1 cases 1-11: the copilot descriptor
// registers at init() and carries the exact literal from CONTRACTS §2.
//
// Two CONTRACTS §2 fields are not representable on the canonical F11 types
// (per F17-T1 instruction 3 "use F12's names — the VALUES are binding"):
//   - ShellSpec has no MaxOutputBytes field; the 32 KiB output cap is F12's
//     CLIResolver constant (F17 SPEC D2, F12 CONTRACTS §4: MaxCLIOutputBytes).
//   - AuthSource.Validate is func(ctx, usage.Credential, *http.Client) error
//     (F11 descriptor.go), so the literal uses validateIdentityHook, which
//     adapts ValidateIdentity's (ctx, token, client) signature.
func TestDescriptorRegistration(t *testing.T) {
	d, err := usage.Get("copilot")
	if err != nil {
		t.Fatalf("usage.Get(\"copilot\") = %v; descriptor must register via init()", err)
	}

	if d.ID != "copilot" {
		t.Errorf("ID = %q, want %q", d.ID, "copilot")
	}
	if d.DisplayName != "GitHub Copilot" {
		t.Errorf("DisplayName = %q, want %q", d.DisplayName, "GitHub Copilot")
	}
	if d.Kind != usage.KindSubscription {
		t.Errorf("Kind = %v, want KindSubscription", d.Kind)
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

	// Windows in descriptor order, all Optional, all UnitRequests.
	wantWindows := []struct {
		id    string
		label string
	}{
		{"premium", "premium interactions"},
		{"chat", "chat"},
		{"completions", "completions"},
	}
	if len(d.Windows) != len(wantWindows) {
		t.Fatalf("len(Windows) = %d, want %d", len(d.Windows), len(wantWindows))
	}
	for i, w := range wantWindows {
		ws := d.Windows[i]
		if ws.ID != w.id {
			t.Errorf("Windows[%d].ID = %q, want %q", i, ws.ID, w.id)
		}
		if ws.Label != w.label {
			t.Errorf("Windows[%d].Label = %q, want %q", i, ws.Label, w.label)
		}
		if ws.Unit != usage.UnitRequests {
			t.Errorf("Windows[%d].Unit = %v, want UnitRequests", i, ws.Unit)
		}
		if !ws.Optional {
			t.Errorf("Windows[%d].Optional = false, want true", i)
		}
	}

	// Auth chain: env, git --global, git --system, gh, device flow.
	// CONTRACTS §2 calls the shell kind "AuthShell"; the canonical F11 name
	// (per F17-T1 instruction 3) is AuthCLIShellOut.
	wantKinds := []usage.AuthKind{
		usage.AuthEnvVar,
		usage.AuthCLIShellOut,
		usage.AuthCLIShellOut,
		usage.AuthCLIShellOut,
		usage.AuthOAuthDeviceFlow,
	}
	if len(d.Auth) != len(wantKinds) {
		t.Fatalf("len(Auth) = %d, want %d", len(d.Auth), len(wantKinds))
	}
	for i, k := range wantKinds {
		if d.Auth[i].Kind != k {
			t.Errorf("Auth[%d].Kind = %v, want %v", i, d.Auth[i].Kind, k)
		}
		if d.Auth[i].Validate == nil {
			t.Errorf("Auth[%d].Validate = nil; every chain entry must carry the identity gate", i)
		}
	}

	// Env entry: COPILOT_API_TOKEN.
	if d.Auth[0].EnvVar != "COPILOT_API_TOKEN" {
		t.Errorf("Auth[0].EnvVar = %q, want %q", d.Auth[0].EnvVar, "COPILOT_API_TOKEN")
	}

	// Shell entries: git --global / git --system / gh; never --local.
	wantShells := []struct {
		cmd  string
		args []string
	}{
		{"git", []string{"config", "--global", "--get", "github.copilot.oauthToken"}},
		{"git", []string{"config", "--system", "--get", "github.copilot.oauthToken"}},
		{"gh", []string{"auth", "token", "--hostname", "github.com"}},
	}
	for i, s := range wantShells {
		sh := d.Auth[i+1].Shell
		if sh == nil {
			t.Fatalf("Auth[%d].Shell = nil", i+1)
		}
		if sh.Command != s.cmd {
			t.Errorf("Auth[%d].Shell.Command = %q, want %q", i+1, sh.Command, s.cmd)
		}
		if len(sh.Args) != len(s.args) {
			t.Errorf("Auth[%d].Shell.Args = %v, want %v", i+1, sh.Args, s.args)
		} else {
			for j := range s.args {
				if sh.Args[j] != s.args[j] {
					t.Errorf("Auth[%d].Shell.Args[%d] = %q, want %q", i+1, j, sh.Args[j], s.args[j])
				}
			}
		}
		if sh.Timeout != 3*time.Second {
			t.Errorf("Auth[%d].Shell.Timeout = %v, want 3s", i+1, sh.Timeout)
		}
		for _, a := range sh.Args {
			if a == "--local" {
				t.Errorf("Auth[%d].Shell.Args contains --local; git config --local must never be used", i+1)
			}
		}
	}

	// Device-flow entry: client id, URLs, scope.
	o := d.Auth[4].OAuth
	if o == nil {
		t.Fatalf("Auth[4].OAuth = nil")
	}
	if o.ClientID != "Iv1.b507a08c87ecfe98" {
		t.Errorf("OAuth.ClientID = %q, want %q", o.ClientID, "Iv1.b507a08c87ecfe98")
	}
	if o.DeviceCodeURL != "https://github.com/login/device/code" {
		t.Errorf("OAuth.DeviceCodeURL = %q, want %q", o.DeviceCodeURL, "https://github.com/login/device/code")
	}
	if o.TokenURL != "https://github.com/login/oauth/access_token" {
		t.Errorf("OAuth.TokenURL = %q, want %q", o.TokenURL, "https://github.com/login/oauth/access_token")
	}
	if o.Scope != "read:user" {
		t.Errorf("OAuth.Scope = %q, want %q", o.Scope, "read:user")
	}
}

// TestConstants covers F17-T1 case 12: endpoint/OAuth constants verbatim from
// copilot.mjs:9-15 plus IdentityUserAgent (SPEC D4).
func TestConstants(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{CopilotUsageURL, "https://api.github.com/copilot_internal/user"},
		{GitHubUserURL, "https://api.github.com/user"},
		{GitHubDeviceCodeURL, "https://github.com/login/device/code"},
		{GitHubDeviceTokenURL, "https://github.com/login/oauth/access_token"},
		{CopilotClientID, "Iv1.b507a08c87ecfe98"},
		{APIVersion, "2025-04-01"},
		{IdentityUserAgent, "which-model/0.4.0"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("constant = %q, want %q", c.got, c.want)
		}
	}
}
