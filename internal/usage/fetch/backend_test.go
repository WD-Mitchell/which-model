//go:build !nousage

package fetch

import (
	"context"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestFetchAllBackendOffSkipsProviders(t *testing.T) {
	old := codexbarFetch
	t.Cleanup(func() { codexbarFetch = old })
	called := false
	codexbarFetch = func(context.Context, string, usage.Source) (usage.Snapshot, error) {
		called = true
		return usage.Snapshot{}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"claude"}, Options{
		Backend: config.UsageBackendOff,
		Enabled: map[string]bool{"claude": true},
	})
	if err != nil || warnings != nil || got != nil || called {
		t.Fatalf("FetchAll(off) = %#v, %v, %v; called=%v", got, warnings, err, called)
	}
}

func TestFetchAllCodexBarBackendSelected(t *testing.T) {
	old := codexbarFetch
	t.Cleanup(func() { codexbarFetch = old })
	codexbarFetch = func(_ context.Context, provider string, _ usage.Source) (usage.Snapshot, error) {
		return usage.Snapshot{Provider: provider, Account: "secret", Plan: "secret-plan"}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"codex"}, Options{
		Backend:      config.UsageBackendCodexBar,
		Enabled:      map[string]bool{"codex": true},
		ShowIdentity: false,
	})
	if err != nil || warnings != nil || len(got) != 1 {
		t.Fatalf("FetchAll(codexbar) = %#v, %v, %v", got, warnings, err)
	}
	if got[0].Provider != "codex" || got[0].Account != "" || got[0].Plan != "" {
		t.Fatalf("FetchAll(codexbar) snapshot = %#v", got[0])
	}
}
