//go:build !nousage

package fetch

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
)

func TestCompanyCodexBarPreflightPrecedesCredentialsAndProcess(t *testing.T) {
	oldPolicy, oldPreflight, oldEnvironment, oldFetch := readCompanyPolicy, codexbarPreflight, codexbarEnvironment, codexbarFetch
	t.Cleanup(func() {
		readCompanyPolicy, codexbarPreflight, codexbarEnvironment, codexbarFetch = oldPolicy, oldPreflight, oldEnvironment, oldFetch
	})
	p := company.Defaults()
	p.AllowedProviders = []string{"antigravity"}
	p.CodexBarInstallations = []company.CodexBarInstallation{{Path: filepath.Join(t.TempDir(), "image.exe")}}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	calls := 0
	codexbarPreflight = func(string) error { calls++; return errors.New("SECRET_CANARY") }
	codexbarEnvironment = func(context.Context, string, Options) map[string]string {
		t.Fatal("credentials resolved before approved verification")
		return nil
	}
	codexbarFetch = func(context.Context, string, usage.Source) (usage.Snapshot, error) {
		t.Fatal("unapproved image launched")
		return usage.Snapshot{}, nil
	}
	got, _, err := FetchAll(context.Background(), []string{"antigravity"}, Options{Backend: config.UsageBackendCodexBar, Enabled: map[string]bool{"antigravity": true}, CacheDir: t.TempDir()})
	if err != nil || calls != 1 || len(got) != 1 || got[0].Failure == nil || strings.Contains(got[0].Failure.Message, "CANARY") || got[0].Failure.Message != companyCodexBarApprovalMessage {
		t.Fatalf("preflight outcome: %+v %v", got, err)
	}
}
func TestCompanyCodexBarCacheOnlyNeverPreflightsOrDelegates(t *testing.T) {
	oldPolicy, oldPreflight, oldEnvironment := readCompanyPolicy, codexbarPreflight, codexbarEnvironment
	t.Cleanup(func() {
		readCompanyPolicy, codexbarPreflight, codexbarEnvironment = oldPolicy, oldPreflight, oldEnvironment
	})
	p := company.Defaults()
	p.AllowedProviders = []string{"antigravity"}
	p.CodexBarInstallations = []company.CodexBarInstallation{{Path: filepath.Join(t.TempDir(), "image.exe")}}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	codexbarPreflight = func(string) error { t.Fatal("cache-only read verified/executed external image"); return nil }
	codexbarEnvironment = func(context.Context, string, Options) map[string]string {
		t.Fatal("cache-only read loaded credential")
		return nil
	}
	dir := t.TempDir()
	if err := (&cache.Store{Dir: dir}).Write("antigravity", usage.Snapshot{Provider: "antigravity", Source: usage.SourceOAuth, Confidence: "live", FetchedAt: time.Now(), UsageKnown: true}); err != nil {
		t.Fatal(err)
	}
	for _, offline := range []bool{true, false} {
		got, _, err := FetchAll(context.Background(), []string{"antigravity"}, Options{Backend: config.UsageBackendCodexBar, Enabled: map[string]bool{"antigravity": true}, CacheDir: dir, Offline: offline})
		if err != nil || len(got) != 1 || got[0].Failure != nil || got[0].Source != usage.SourceCache {
			t.Fatalf("cache outcome: %+v %v", got, err)
		}
	}
}

func TestCompanyCodexBarLocalCacheMatchesCLIMode(t *testing.T) {
	oldPolicy, oldPreflight := readCompanyPolicy, codexbarPreflight
	t.Cleanup(func() { readCompanyPolicy, codexbarPreflight = oldPolicy, oldPreflight })
	for _, provider := range []string{"antigravity", "windsurf"} {
		t.Run(provider, func(t *testing.T) {
			p := company.Defaults()
			p.AllowedProviders = []string{provider}
			p.CodexBarInstallations = []company.CodexBarInstallation{{Path: filepath.Join(t.TempDir(), "image.exe")}}
			readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
			codexbarPreflight = func(string) error { t.Fatal("matching local cache triggered a child"); return nil }
			dir := t.TempDir()
			if err := (&cache.Store{Dir: dir}).Write(provider, usage.Snapshot{Provider: provider, Source: usage.SourceLocal, FetchedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			got, _, err := FetchAll(context.Background(), []string{provider}, Options{Backend: config.UsageBackendCodexBar, Enabled: map[string]bool{provider: true}, CacheDir: dir, Source: usage.SourceCLI})
			if err != nil || len(got) != 1 || got[0].Failure != nil || got[0].Source != usage.SourceCache {
				t.Fatalf("matching CLI-mode cache: %+v %v", got, err)
			}
		})
	}
}
