//go:build !nousage

package whichmodel

import (
	"context"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "which-model-tests-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(home)
	if err := os.Setenv("HOME", home); err != nil {
		panic(err)
	}
	configDir := filepath.Join(home, "Library", "Application Support", "which-model")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[usage]\nbackend = \"native\"\n"), 0o600); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestRunUsageBackendOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[usage]\nbackend = \"off\"\n[providers.claude]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	err := RunUsage(UsageArgs{Providers: []string{"claude"}, ConfigPath: path}, &out, &errOut)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "usage is disabled by [usage] backend = off") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
func TestRunUsageCodexBarBackendSelection(t *testing.T) {
	old := fetchAllFunc
	t.Cleanup(func() { fetchAllFunc = old })
	var got FetchAllOptions
	fetchAllFunc = func(_ context.Context, opts FetchAllOptions) (*FetchResult, error) {
		got = opts
		return &FetchResult{Snapshots: []usage.Snapshot{{Provider: "claude"}}}, nil
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[usage]\nbackend = \"codexbar\"\n[providers.claude]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut strings.Builder
	if err := RunUsage(UsageArgs{Providers: []string{"claude"}, ConfigPath: path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got.Backend != config.UsageBackendCodexBar {
		t.Fatalf("fetch backend = %q, want %q", got.Backend, config.UsageBackendCodexBar)
	}
}
func TestRunUsageNativeBackendRejectsCodexBarOnlyProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[usage]\nbackend = \"native\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RunUsage(UsageArgs{Providers: []string{"openai"}, ConfigPath: path}, nil, nil)
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "valid providers: claude, codex, copilot") {
		t.Fatalf("err = %v, exit = %d", err, ExitCodeFor(err))
	}
}
