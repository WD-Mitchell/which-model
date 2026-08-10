//go:build !nousage

package fetch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
)

func TestFetchAllDefaultDeny(t *testing.T) {
	old := fetchProvider
	t.Cleanup(func() { fetchProvider = old })
	called := false
	fetchProvider = func(context.Context, string) (usage.Snapshot, error) {
		called = true
		return usage.Snapshot{}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"claude"}, Options{})
	if err != nil || warnings != nil || got != nil || called {
		t.Fatalf("FetchAll() = %#v, %v, %v; want default-deny no-op", got, warnings, err)
	}
}

func TestFetchAllSortsAndRedacts(t *testing.T) {
	old := fetchProvider
	t.Cleanup(func() { fetchProvider = old })
	fetchProvider = func(_ context.Context, id string) (usage.Snapshot, error) {
		return usage.Snapshot{Provider: id, Account: "account", Plan: "plan", FetchedAt: time.Unix(1, 0)}, nil
	}
	got, warnings, err := FetchAll(context.Background(), []string{"codex", "claude"}, Options{Enabled: map[string]bool{"codex": true, "claude": true}})
	if err != nil || warnings != nil || len(got) != 2 {
		t.Fatalf("FetchAll() = %#v, %v, %v", got, warnings, err)
	}
	if got[0].Provider != "claude" || got[1].Provider != "codex" {
		t.Fatalf("providers = [%s, %s], want sorted", got[0].Provider, got[1].Provider)
	}
	for _, snap := range got {
		if snap.Account != "" || snap.Plan != "" {
			t.Fatalf("identity not redacted: %#v", snap)
		}
	}
}

func TestFetchAllMapsBinaryNotFound(t *testing.T) {
	old := fetchProvider
	t.Cleanup(func() { fetchProvider = old })
	fetchProvider = func(context.Context, string) (usage.Snapshot, error) {
		return usage.Snapshot{}, &codexbar.BinaryNotFoundError{}
	}
	got, _, err := FetchAll(context.Background(), []string{"claude"}, Options{Enabled: map[string]bool{"claude": true}})
	if err != nil || len(got) != 1 || got[0].Failure == nil {
		t.Fatalf("FetchAll() = %#v, %v", got, err)
	}
	if got[0].Failure.Code != "provider_status" || got[0].Failure.Message != "codexbar CLI not found; install from https://github.com/steipete/CodexBar" {
		t.Fatalf("failure = %#v", got[0].Failure)
	}
}

func TestFetchAllPassesSource(t *testing.T) {
	old := fetchProviderWithSource
	t.Cleanup(func() { fetchProviderWithSource = old })
	called := usage.Source("")
	fetchProviderWithSource = func(_ context.Context, id string, source usage.Source) (usage.Snapshot, error) {
		called = source
		return usage.Snapshot{Provider: id}, nil
	}
	got, _, err := FetchAll(context.Background(), []string{"claude"}, Options{Enabled: map[string]bool{"claude": true}, Source: usage.SourceWeb})
	if err != nil || len(got) != 1 || called != usage.SourceWeb {
		t.Fatalf("FetchAll() = %#v, called source = %q, err = %v", got, called, err)
	}
}

func TestFetchAllPropagatesParentCancellation(t *testing.T) {
	old := fetchProvider
	t.Cleanup(func() { fetchProvider = old })
	fetchProvider = func(ctx context.Context, id string) (usage.Snapshot, error) {
		<-ctx.Done()
		return usage.Snapshot{Provider: id}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := FetchAll(ctx, []string{"claude"}, Options{Enabled: map[string]bool{"claude": true}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchAll() error = %v, want context.Canceled", err)
	}
}
