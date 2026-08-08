//go:build !nousage

package whichmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestAuthStatusJSONGolden(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	future := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	resolveFirstFunc = func(id string) (AuthResolved, error) {
		if id == "claude" {
			return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &future}, nil
		}
		return AuthResolved{}, errNoCredential
	}
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude", "copilot"}, JSON: true}, &out, &errOut)
	if ExitCodeFor(err) != 5 {
		t.Fatalf("exit = %d, err = %v", ExitCodeFor(err), err)
	}
	var got map[string]any
	if json.Unmarshal([]byte(out.String()), &got) != nil {
		t.Fatalf("stdout = %q", out.String())
	}
	if got["schema_version"] != "2.0" || got["usage_enabled"] != true {
		t.Fatalf("envelope = %v", got)
	}
	providers := got["providers"].([]any)
	claude := providers[0].(map[string]any)
	copilot := providers[1].(map[string]any)
	if claude["provider"] != "claude" || claude["status"] != "ok" || claude["source"] != "oauth" || claude["expires_at"] != "2026-09-01T00:00:00Z" || claude["fingerprint"] != Fingerprint("tok") {
		t.Fatalf("claude = %v", claude)
	}
	if copilot["provider"] != "copilot" || copilot["status"] != "missing" || copilot["source"] != nil || copilot["fingerprint"] != nil {
		t.Fatalf("copilot = %v", copilot)
	}
	if _, ok := copilot["expires_at"]; ok {
		t.Fatalf("missing expiry present: %v", copilot)
	}
}

func TestAuthStatusMissingExitFiveWithReport(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{}, errNoCredential }
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"copilot"}, JSON: true}, &out, &errOut)
	if ExitCodeFor(err) != 5 || out.Len() == 0 {
		t.Fatalf("err = %v, exit = %d, out = %q", err, ExitCodeFor(err), out.String())
	}
}

func TestAuthStatusAllOKExitZero(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: "tok"}, nil }
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
}

func TestAuthStatusExpiredExitFive(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	past := time.Now().Add(-time.Hour)
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &past}, nil }
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut)
	if ExitCodeFor(err) != 5 || out.Len() == 0 {
		t.Fatalf("err = %v, exit = %d, out = %q", err, ExitCodeFor(err), out.String())
	}
}

func TestAuthStatusDisabledFlag(t *testing.T) {
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, NoUsage: true}, &out, &errOut)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "usage is disabled by --no-usage") || out.Len() != 0 {
		t.Fatalf("err = %v, exit = %d, out = %q", err, ExitCodeFor(err), out.String())
	}
}

func TestAuthStatusDisabledConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[usage]\nenabled = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, ConfigPath: path}, &out, &errOut)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "[usage] enabled = false") || out.Len() != 0 {
		t.Fatalf("err = %v, exit = %d, out = %q", err, ExitCodeFor(err), out.String())
	}
}
