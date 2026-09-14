//go:build !nousage

package cache

import (
	"encoding/json"
	"errors"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompanyCacheIdentityAndExpiry(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	dir := t.TempDir()
	s := &Store{Dir: dir}
	snap := usage.Snapshot{Provider: "codex", Account: "ACCOUNT_CANARY", Plan: "PLAN_CANARY", FetchedAt: time.Now(), Windows: []usage.Window{{ID: "primary", Label: "LABEL_CANARY", ResetHint: "HINT_CANARY", Unit: usage.UnitPercent, UsageKnown: true}}, UsageKnown: true}
	if err := s.Write("codex", snap); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "codex.json"))
	if strings.Contains(string(data), "CANARY") {
		t.Fatal("identity persisted")
	}
	got, _, err := s.Read("codex", time.Hour)
	if err != nil || got.Account != "" || len(got.Windows) != 1 {
		t.Fatalf("safe cache read: %v", err)
	}
	// Legacy identity is scrubbed without renewing its original retention clock.
	oldTime := time.Now().Add(-23 * time.Hour)
	data, _ = json.Marshal(cacheFile{Snapshot: snap, FetchedAt: oldTime})
	os.WriteFile(filepath.Join(dir, "codex.json"), data, 0600)
	got = s.OfflineRead("codex", time.Minute)
	if got.Account != "" || !got.Stale {
		t.Fatal("offline transition did not minimize stale data")
	}
	data, _ = os.ReadFile(filepath.Join(dir, "codex.json"))
	if strings.Contains(string(data), "CANARY") || !strings.Contains(string(data), oldTime.Format(time.RFC3339Nano)) {
		t.Fatal("legacy identity/time transition failed")
	}
	data, _ = json.Marshal(cacheFile{Snapshot: snap, FetchedAt: time.Now().Add(-25 * time.Hour)})
	os.WriteFile(filepath.Join(dir, "codex.json"), data, 0600)
	if _, _, err := s.Read("codex", time.Minute); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expired cache is eligible: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "codex.json")); !os.IsNotExist(err) {
		t.Fatal("expired data not deleted")
	}
}

func TestCompanyCacheRejectsRedirectedCredential(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	dir := t.TempDir()
	target := filepath.Join(dir, "provider-owned")
	os.WriteFile(target, []byte("CREDENTIAL_CANARY"), 0600)
	if err := os.Symlink(target, filepath.Join(dir, "codex.json")); err != nil {
		t.Skip("symlinks unavailable")
	}
	s := &Store{Dir: dir}
	got := s.OfflineRead("codex", time.Hour)
	if got.Failure == nil {
		t.Fatal("redirected cache was eligible")
	}
	if err := s.Write("codex", usage.Snapshot{Provider: "codex"}); err == nil {
		t.Fatal("redirected entry overwritten")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "CREDENTIAL_CANARY" {
		t.Fatal("provider-owned target modified")
	}
}
