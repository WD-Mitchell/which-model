package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

func writeCodexModelsFixture(t *testing.T, svc *Services, body string) {
	t.Helper()
	path := filepath.Join(svc.harnessHome, ".codex", "models_cache.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogListsUnbenchmarkedModelsImmediately(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`[gui]
only_enabled_providers = true
[providers.codex]
enabled = true
[providers.zai]
enabled = true
`))
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{{Provider: "zai", ModelID: "glm-5.3", Name: "GLM-5.3", EffortLevels: []string{"high"}}})
	writeCodexModelsFixture(t, svc, `{"models":[{"slug":"gpt-6-astra","display_name":"GPT-6-Astra","visibility":"list","supported_reasoning_levels":[{"effort":"high"},{"effort":"max"},{"effort":"ultra"}]},{"slug":"hidden","visibility":"hide"}]}`)
	list, err := svc.Catalog().Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]CatalogModel{}
	for _, m := range list {
		found[m.ModelID] = m
	}
	for id, maker := range map[string]string{"gpt-6-astra": "OpenAI", "glm-5.3": "Z.AI"} {
		m, ok := found[id]
		if !ok {
			t.Errorf("missing unbenchmarked model %s", id)
			continue
		}
		if m.Maker != maker || m.Intelligence != nil || m.Cost != nil || m.Speed != nil || m.ProviderCount != 1 {
			t.Errorf("unexpected model: %+v", m)
		}
		card, err := svc.Catalog().Model(context.Background(), m.ModelName)
		if err != nil {
			t.Fatal(err)
		}
		if card.InCatalog || len(card.Providers) != 1 {
			t.Errorf("unbenchmarked card: %+v", card)
		}
	}
	if _, ok := found["hidden"]; ok {
		t.Fatal("hidden Codex model was listed")
	}
	// A new local release is visible on the next read, without waiting for a
	// score refresh or restarting the service.
	writeCodexModelsFixture(t, svc, `{"models":[{"slug":"gpt-next","display_name":"GPT Next","visibility":"list"}]}`)
	list, err = svc.Catalog().Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.ModelID == "gpt-next" {
			return
		}
	}
	t.Fatal("new local model did not appear on the next read")
}

func TestCatalogDiscoveryKeepsUnbenchmarkedLiveModels(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`[usage]
backend = "native"
[providers.cursor]
enabled = true
`))
	stubCatalogRepoFromCache(t, svc)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{{Provider: "openai", ModelID: "old", Name: "Old Model"}})
	previous := discoverLiveProviderModels
	discoverLiveProviderModels = func(context.Context, string) []routing.ModelEntry {
		return []routing.ModelEntry{{ModelID: "brand-new", Name: "Brand New", Reasoning: []string{"high"}}}
	}
	t.Cleanup(func() { discoverLiveProviderModels = previous })
	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatal(err)
	}
	list, err := svc.Catalog().Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.ModelID == "brand-new" && m.Intelligence == nil {
			return
		}
	}
	t.Fatalf("live model without benchmark scores was discarded; inventory=%#v, list=%#v", readProviderInventory(svc.paths.CacheDir), list)
}

func TestMakerUsesCompaniesNotUnknownModelNames(t *testing.T) {
	for name, want := range map[string]string{"GLM-5.3": "Z.AI", "GLM 5.3 Flash": "Z.AI", "z-ai/glm-5.3": "Z.AI", "Unannounced 7.0": "Other"} {
		if got := extractMaker(name); got != want {
			t.Errorf("extractMaker(%q)=%q, want %q", name, got, want)
		}
	}
}
