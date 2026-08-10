//go:build !nousage

package whichmodel

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func usageTestConfig(t *testing.T, body string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.LoadOptions{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestResolveProvidersValidationMoveOver(t *testing.T) {
	err := RunUsage(UsageArgs{Providers: []string{"not-a-provider"}}, nil, nil)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}

func TestResolveProvidersAllEnabled(t *testing.T) {
	cfg := usageTestConfig(t, "[providers.claude]\nenabled = true\n[providers.codex]\nenabled = true\n")
	got, err := resolveProviders(UsageArgs{All: true}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("providers = %v", got)
	}
}

func TestResolveProvidersAllOnlyEnabled(t *testing.T) {
	cfg := usageTestConfig(t, "[providers.claude]\nenabled = true\n[providers.codex]\nenabled = false\n[providers.copilot]\n")
	got, err := resolveProviders(UsageArgs{All: true}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"claude"}) {
		t.Fatalf("providers = %v", got)
	}
}

func TestResolveProvidersPreservesPositionalOrder(t *testing.T) {
	got, err := resolveProviders(UsageArgs{Providers: []string{"codex", "claude"}}, usageTestConfig(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"codex", "claude"}) {
		t.Fatalf("providers = %v", got)
	}
}

func TestDisplayNamePassThrough(t *testing.T) {
	if got := displayName("claude"); got != "claude" {
		t.Fatalf("displayName(claude) = %q", got)
	}
	if got := displayName("no-such-id"); got != "no-such-id" {
		t.Fatalf("displayName(no-such-id) = %q", got)
	}
}

func TestResolveProvidersAllEmpty(t *testing.T) {
	got, err := resolveProviders(UsageArgs{All: true}, usageTestConfig(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		got = []string{}
	}
	if !reflect.DeepEqual(got, []string{}) {
		t.Fatalf("providers = %v", got)
	}
}
