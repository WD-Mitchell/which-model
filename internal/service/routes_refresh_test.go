package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
)

func TestRefreshRoutesBuildsFromCatalogue(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
`))
	stubCatalogRepoFromCache(t, svc)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{
		Provider:     "anthropic",
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{
		Provider:     "anthropic",
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})

	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatalf("Detail after RefreshRoutes: %v", err)
	}
	if len(detail.Models) == 0 {
		t.Fatal("Detail.Models is empty after RefreshRoutes, want catalogue-joined routes")
	}
	found := false
	for _, m := range detail.Models {
		if m.ModelID == "claude-opus-5" && len(m.Levels) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Detail.Models = %+v, want claude-opus-5 with levels", detail.Models)
	}
	sawRoutes := false
	for _, ev := range rec.Events() {
		if ev.Event == EventConfigChanged {
			sawRoutes = true
		}
	}
	if !sawRoutes {
		t.Fatalf("events after RefreshRoutes = %+v, want config:changed", rec.Events())
	}
}

func TestRefreshRoutesFetchesWhenCacheMissing(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[providers.claude]
enabled = true
`))
	stubCatalogRepoFromCache(t, svc)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{
		Provider:     "anthropic",
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})
	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}
	if _, err := os.Stat(modelsDevCachePath(svc.paths.CacheDir)); err != nil {
		t.Fatalf("models.dev cache after fetch: %v", err)
	}
	detail, err := svc.Providers().Detail(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Models) == 0 {
		t.Fatal("Detail.Models is empty after fetched catalogue")
	}
}

func TestRefreshRoutesInvokesCatalogHook(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[gui]
use_local_aa = true

[providers.claude]
enabled = true
`))
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{
		Provider:     "anthropic",
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})
	called := 0
	svc.SetCatalogRefresh(func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		called++
		return nil
	})
	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}
	if called != 1 {
		t.Fatalf("catalog hook calls = %d, want 1", called)
	}
	sawCatalog := false
	for _, ev := range rec.Events() {
		if ev.Event == EventCatalogChanged {
			sawCatalog = true
		}
	}
	if !sawCatalog {
		t.Fatalf("events after RefreshRoutes = %+v, want catalog:changed", rec.Events())
	}
}

func TestRefreshRoutesCatalogHookErrorStopsJoin(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"

[gui]
use_local_aa = true

[providers.claude]
enabled = true
`))
	fetched := 0
	prev := fetchModelsDevCatalogue
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		fetched++
		return nil, nil
	}
	t.Cleanup(func() { fetchModelsDevCatalogue = prev })
	svc.SetCatalogRefresh(func(context.Context) error {
		return context.Canceled
	})
	if err := svc.Providers().RefreshRoutes(context.Background()); err == nil {
		t.Fatal("RefreshRoutes() error = nil, want catalog hook failure")
	}
	if fetched != 0 {
		t.Fatalf("models.dev fetch calls = %d, want 0 after hook failure", fetched)
	}
}

func TestRefreshRoutesDefaultPullsRepoNotHook(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`
[usage]
backend = "native"
`))
	hookCalls := 0
	svc.SetCatalogRefresh(func(context.Context) error {
		hookCalls++
		return context.Canceled
	})
	stubCatalogRepoFromCache(t, svc)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{
		Provider:     "anthropic",
		ModelID:      "claude-opus-5",
		Name:         "Claude Opus 5",
		EffortLevels: []string{"max"},
	}})
	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}
	if hookCalls != 0 {
		t.Fatalf("catalog hook calls = %d, want 0 when use_local_aa is off", hookCalls)
	}
}

func stubModelsDevFetch(t *testing.T, models []modelsdev.ProviderModel) {
	t.Helper()
	prev := fetchModelsDevCatalogue
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		return models, nil
	}
	t.Cleanup(func() { fetchModelsDevCatalogue = prev })
}

func seedModelsDevCache(t *testing.T, svc *Services, models []modelsdev.ProviderModel) {
	t.Helper()
	data, err := json.Marshal(models)
	if err != nil {
		t.Fatal(err)
	}
	path := modelsDevCachePath(svc.paths.CacheDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestModelsDevExplicitRefreshReplacesValidCache(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(providersFixture))
	stubCatalogRepoFromCache(t, svc)
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{Provider: "anthropic", ModelID: "old-model", Name: "Old"}})
	old := fetchModelsDevCatalogue
	t.Cleanup(func() { fetchModelsDevCatalogue = old })
	calls := 0
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		calls++
		return []modelsdev.ProviderModel{{Provider: "anthropic", ModelID: "claude-opus-5", Name: "Claude Opus 5", EffortLevels: []string{"max"}}}, nil
	}
	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, ok := readModelsDevCache(modelsDevCachePath(svc.paths.CacheDir))
	if !ok || len(got) != 1 || got[0].ModelID != "claude-opus-5" || calls != 1 {
		t.Fatalf("catalogue=%+v calls=%d", got, calls)
	}
	if _, err := svc.Providers().List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Providers().Detail(context.Background(), "claude"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("read-only fetches=%d", calls-1)
	}
}

func TestModelsDevRefreshFailureIsReportedWithoutTouchingCache(t *testing.T) {
	svc, _ := newTestServices(t)
	path := modelsDevCachePath(svc.paths.CacheDir)
	// Old schema must also be read without hidden network work.
	if err := os.WriteFile(path, []byte(`[{"Provider":"anthropic","ModelID":"cached","Name":"Cached"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := fetchModelsDevCatalogue
	t.Cleanup(func() { fetchModelsDevCatalogue = old })
	calls := 0
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		calls++
		return nil, fmt.Errorf("synthetic fetch failure")
	}
	if _, ok := readModelsDevCache(path); !ok || calls != 0 {
		t.Fatalf("cache read calls=%d", calls)
	}
	got, err := svc.Providers().loadOrFetchModelsDev(context.Background())
	if err == nil || len(got) != 0 || calls != 1 {
		t.Fatalf("refresh=%+v error=%v calls=%d", got, err, calls)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("fallback touched cache")
	}
	if err = os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Providers().loadOrFetchModelsDev(context.Background()); err == nil {
		t.Fatal("corrupt cache hid fetch error")
	}
}

// Refresh must update the surfaces consumed by Settings, not only the cache bytes.
func TestModelsDevRefreshUpdatesDetailPricesAndRemovesCatalogueModel(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(providersFixture))
	stubCatalogRepoFromCache(t, svc)
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{
		{Provider: "anthropic", ModelID: "claude-opus-5", Name: "Claude Opus 5", EffortLevels: []string{"max"}, InputCostUSDPerM: fptr(15), OutputCostUSDPerM: fptr(75)},
		{Provider: "anthropic", ModelID: "retired-test-model", Name: "Retired Test Model"},
	})
	ctx := context.Background()
	assertDetail := func(wantRemoved bool, wantInput, wantOutput float64) {
		t.Helper()
		detail, err := svc.Providers().Detail(ctx, "claude")
		if err != nil {
			t.Fatal(err)
		}
		hasRetired, hasRetained := false, false
		for _, model := range detail.Models {
			if model.ModelID == "retired-test-model" {
				hasRetired = true
			}
			if model.ModelID == "claude-opus-5" {
				hasRetained = true
			}
		}
		if hasRetired == wantRemoved || !hasRetained {
			t.Fatalf("provider detail has retired=%v retained=%v, removed=%v", hasRetired, hasRetained, wantRemoved)
		}
		model, err := svc.Catalog().Model(ctx, "Claude Opus 5")
		if err != nil {
			t.Fatal(err)
		}
		for _, provider := range model.Providers {
			if provider.Provider != "claude" || provider.ModelID != "claude-opus-5" {
				continue
			}
			if provider.InputCostUSDPerM == nil || provider.OutputCostUSDPerM == nil || *provider.InputCostUSDPerM != wantInput || *provider.OutputCostUSDPerM != wantOutput {
				t.Fatalf("model detail prices=%v/%v want=%v/%v", provider.InputCostUSDPerM, provider.OutputCostUSDPerM, wantInput, wantOutput)
			}
			return
		}
		t.Fatalf("model detail has no Claude price row: %+v", model.Providers)
	}
	old := fetchModelsDevCatalogue
	t.Cleanup(func() { fetchModelsDevCatalogue = old })
	calls := 0
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		calls++
		return []modelsdev.ProviderModel{{Provider: "anthropic", ModelID: "claude-opus-5", Name: "Claude Opus 5", EffortLevels: []string{"max"}, InputCostUSDPerM: fptr(3), OutputCostUSDPerM: fptr(15)}}, nil
	}
	assertDetail(false, 15, 75)
	if calls != 0 {
		t.Fatalf("reading cached detail performed %d fetches", calls)
	}
	if err := svc.Providers().RefreshRoutes(ctx); err != nil {
		t.Fatal(err)
	}
	assertDetail(true, 3, 15)
	if calls != 1 {
		t.Fatalf("refresh plus detail reads performed %d fetches, want one", calls)
	}
}

func TestModelsDevRefreshCancellationDoesNotFallBackOrWrite(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx, cancel := context.WithCancel(context.Background())
	prior := fetchModelsDevCatalogue
	t.Cleanup(func() { fetchModelsDevCatalogue = prior })
	fetchModelsDevCatalogue = func(context.Context) ([]modelsdev.ProviderModel, error) {
		cancel()
		return []modelsdev.ProviderModel{{Provider: "claude", ModelID: "cancelled"}}, nil
	}
	_, err := svc.Providers().loadOrFetchModelsDev(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(modelsDevCachePath(svc.paths.CacheDir)); !os.IsNotExist(err) {
		t.Fatalf("cancelled fetch cached: %v", err)
	}
}
