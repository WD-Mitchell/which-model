package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Provider accounts must survive a load → marshal → load cycle. The array of
// tables has to render as [[providers.<id>.accounts]]; an earlier version built
// it as []any, which renderSection treated as a scalar and emitted as a
// top-level [[accounts]] table — silently reparenting it on the next load.
func TestProviderAccountsRoundTrip(t *testing.T) {
	src := `
[providers.claude]
enabled = true
priority = 1

[[providers.claude.accounts]]
name = "Work"
kind = "oauth"
ref = "~/.claude/.credentials.json"

[[providers.claude.accounts]]
name = "Personal"
kind = "token"
ref = "WM_CLAUDE_TOKEN"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if got := len(cfg.Providers["claude"].Accounts); got != 2 {
		t.Fatalf("accounts = %d, want 2", got)
	}

	out, err := cfg.MarshalTOML()
	if err != nil {
		t.Fatalf("MarshalTOML: %v", err)
	}
	if !strings.Contains(string(out), "[[providers.claude.accounts]]") {
		t.Errorf("accounts not scoped to the provider:\n%s", out)
	}
	if strings.Contains(string(out), "\n[[accounts]]") {
		t.Errorf("accounts rendered at top level:\n%s", out)
	}

	round := filepath.Join(dir, "round.toml")
	if err := os.WriteFile(round, out, 0o600); err != nil {
		t.Fatal(err)
	}
	back, err := LoadFile(round)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := back.Providers["claude"].Accounts
	if len(got) != 2 || got[0].Name != "Work" || got[0].Kind != "oauth" ||
		got[1].Name != "Personal" || got[1].Ref != "WM_CLAUDE_TOKEN" {
		t.Errorf("round-tripped accounts = %+v", got)
	}
}
