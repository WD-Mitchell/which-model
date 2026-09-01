package service

import (
	"context"
	"encoding/json"
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

func stubModelsDevFetch(t *testing.T, models []modelsdev.ProviderModel) {
	t.Helper()
	prev := fetchModelsDevCatalogue
	fetchModelsDevCatalogue = func() ([]modelsdev.ProviderModel, error) {
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
