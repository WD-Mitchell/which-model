//go:build !nousage

package whichmodel

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func authStatusConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveStatusesOK(t *testing.T) {
	future := time.Now().Add(time.Hour)
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &future, Account: "user@x"}, nil
	}
	entries, err := resolveStatuses(AuthStatusArgs{Providers: []string{"claude"}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
	entry := entries[0]
	if entry.Provider != "claude" || entry.Status != "ok" || entry.Source == nil || *entry.Source != "oauth" || entry.ExpiresAt != &future || entry.Fingerprint == nil || *entry.Fingerprint != Fingerprint("tok") {
		t.Fatalf("entry = %#v", entry)
	}
}

func TestResolveStatusesMissing(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{}, errNoCredential }
	entries, err := resolveStatuses(AuthStatusArgs{Providers: []string{"claude"}})
	if err != nil || len(entries) != 1 || entries[0].Status != "missing" || entries[0].Source != nil || entries[0].Fingerprint != nil || entries[0].ExpiresAt != nil {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
}

func TestResolveStatusesExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &past}, nil
	}
	entries, err := resolveStatuses(AuthStatusArgs{Providers: []string{"claude"}})
	if err != nil || entries[0].Status != "expired" || entries[0].ExpiresAt != &past {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
}

func TestRunAuthStatusAllExpansion(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(id string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: id}, nil }
	path := authStatusConfig(t, "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n[providers.codex]\nenabled = true\n")
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{All: true, JSON: true, ConfigPath: path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"provider": "claude"`) || !strings.Contains(out.String(), `"provider": "codex"`) {
		t.Fatalf("stdout = %q", out.String())
	}
	if strings.Index(out.String(), `"provider": "claude"`) > strings.Index(out.String(), `"provider": "codex"`) {
		t.Fatalf("providers out of order: %q", out.String())
	}
}

func TestRunAuthStatusExplicitProvider(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(id string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: id}, nil }
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"copilot"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), `"provider"`) != 1 || !strings.Contains(out.String(), `"copilot"`) {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestRunAuthStatusResolverError(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{}, errors.New("resolver failed") }
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}}, &out, &errOut)
	if ExitCodeFor(err) != 1 || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
