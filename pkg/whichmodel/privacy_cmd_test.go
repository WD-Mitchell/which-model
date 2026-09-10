//go:build !nousage

package whichmodel

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivacyExplicitPurgeReportsPartialFailure(t *testing.T) {
	home := t.TempDir()
	for _, key := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(key, home)
	}
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	layout, err := privacy.DefaultLayout()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(layout.StateDir, "pick", "history.jsonl")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := captureExecute(t, []string{"privacy", "purge", "--category", "history", "--project-root", home})
	if code == 0 || !strings.Contains(out, `"failed_files":1`) {
		t.Fatalf("purge did not report failure: exit=%d out=%s stderr=%s", code, out, stderr)
	}
	if strings.Contains(out, home) || strings.Contains(stderr, home) {
		t.Fatal("failure exposed local identity path")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("LEGACY_IDENTITY_CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr = captureExecute(t, []string{"privacy", "purge", "--category", "history", "--project-root", home})
	if code != 0 || !strings.Contains(out, `"deleted_files":1`) {
		t.Fatalf("purge failed: exit=%d out=%s stderr=%s", code, out, stderr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("purged history still exists")
	}
	if strings.Contains(out, "CANARY") || strings.Contains(stderr, "CANARY") {
		t.Fatal("purge echoed content")
	}
}
