package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

const modelsDevCacheFile = "modelsdev_providers.json"

// fetchModelsDevCatalogue is the network seam for RefreshRoutes. Tests replace
// it so a missing cache never hits models.dev.
var fetchModelsDevCatalogue = fetchModelsDevCatalogueLive

func fetchModelsDevCatalogueLive(ctx context.Context) ([]modelsdev.ProviderModel, error) {
	// models.dev api.json is ~4 MB (F08); the default 256 KiB bound is too small.
	client := httpkit.NewClient(httpkit.WithTimeout(30*time.Second), httpkit.WithMaxBytes(16<<20))
	return modelsdev.FetchModelsDevProvidersFromContext(ctx, client, modelsdev.ProvidersURL)
}

func modelsDevCachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "catalog", modelsDevCacheFile)
}

// RefreshRoutes rebuilds the route table the same way `which-model routes
// refresh` does: a freshly fetched models.dev catalogue joined to the
// scores CSV, preserving user-declared routes. Settings calls this after a
// successful sign-in and from the signed-in "Refresh models" button.
//
// When catalogRefresh is set (desktop host) AND the user opted into a local
// Artificial Analysis key, scores are rebuilt via that hook. Otherwise
// scores are pulled from the configured GitHub repo (default: the main
// which-model repository). Login still succeeds if this step fails.
func (p *ProviderService) RefreshRoutes(ctx context.Context) error {
	p.s.dataRefreshMu.Lock()
	defer p.s.dataRefreshMu.Unlock()
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	if err := p.s.refreshCatalogSource(ctx); err != nil {
		return toErrorDTO(fmt.Errorf("refresh data: %w", err))
	}
	if err := p.s.reloadCatalog(); err != nil {
		return toErrorDTO(fmt.Errorf("reload catalogue: %w", err))
	}
	catalogue, err := p.loadOrFetchModelsDev(ctx)
	if err != nil {
		return toErrorDTO(fmt.Errorf("refresh models: %w", err))
	}
	p.s.mu.RLock()
	liveProviderIDs := p.liveModelProviderIDsLocked()
	p.s.mu.RUnlock()
	liveModels := make(map[string][]routing.ModelEntry, len(liveProviderIDs))
	for _, id := range liveProviderIDs {
		var models []routing.ModelEntry
		if id == "codex" {
			models = p.s.codexModels()
		} else {
			models = discoverLiveProviderModels(ctx, id)
		}
		if models != nil {
			liveModels[id] = models
		}
	}
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	if err := writeProviderInventory(p.s.paths.CacheDir, liveModels); err != nil {
		return toErrorDTO(err)
	}
	p.s.mu.RLock()
	input := p.routeProductionInputLocked(catalogue, liveModels)
	p.s.mu.RUnlock()

	result, err := routing.ProduceRoutes(input)
	if err != nil {
		return toErrorDTO(err)
	}
	routes := result.Routes
	if routes == nil {
		routes = []routing.Route{}
	}
	hash, err := csvstore.ProvenanceHash(filepath.Join(p.s.paths.CacheDir, "catalog", "available_model_scores.csv"))
	if err != nil {
		return toErrorDTO(err)
	}
	refreshed := make(map[string]string, len(input.Providers))
	for _, provider := range input.Providers {
		refreshed[provider.Provider] = time.Now().UTC().Format(time.RFC3339)
	}
	table := routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		ScoresHash:    hash,
		RefreshedAt:   refreshed,
		Routes:        routes,
	}

	p.s.mu.Lock()
	if err := ctx.Err(); err != nil {
		p.s.mu.Unlock()
		return toErrorDTO(err)
	}
	if err := routing.SaveTable(filepath.Join(p.s.paths.CacheDir, "routes.json"), table); err != nil {
		p.s.mu.Unlock()
		return toErrorDTO(err)
	}
	p.s.routes = table
	p.s.mu.Unlock()
	p.s.emit(EventCatalogChanged, map[string]any{})
	p.s.emit(EventConfigChanged, map[string]string{"section": "routes"})
	return nil
}

func (p *ProviderService) loadOrFetchModelsDev(ctx context.Context) ([]modelsdev.ProviderModel, error) {
	path := modelsDevCachePath(p.s.paths.CacheDir)
	catalogue, err := fetchModelsDevCatalogue(ctx)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err == nil && len(catalogue) == 0 {
		err = fmt.Errorf("models.dev catalogue is empty")
	}
	if err != nil {
		return nil, err
	}
	if err := writeModelsDevCache(path, catalogue); err != nil {
		return nil, err
	}
	return catalogue, nil
}

func readModelsDevCache(path string) ([]modelsdev.ProviderModel, bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var catalogue []modelsdev.ProviderModel
	if err := json.Unmarshal(data, &catalogue); err != nil || len(catalogue) == 0 {
		return nil, false
	}

	return catalogue, true
}

func writeModelsDevCache(path string, catalogue []modelsdev.ProviderModel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(catalogue)
	if err != nil {
		return err
	}
	return config.AtomicWriteFile(path, data)
}

func (p *ProviderService) routeProductionInputLocked(catalogue []modelsdev.ProviderModel, liveModels map[string][]routing.ModelEntry) routing.Input {
	bySlug := make(map[string][]routing.ModelEntry, len(catalogue))
	for _, m := range catalogue {
		bySlug[m.Provider] = append(bySlug[m.Provider], routing.ModelEntry{
			ModelID:   m.ModelID,
			Name:      m.Name,
			Reasoning: m.EffortLevels,
		})
	}

	input := routing.Input{
		Providers:   make([]routing.ProviderInput, 0),
		CatalogRows: make([]identity.Identity, 0, len(p.s.scores)),
	}
	for _, row := range p.s.scores {
		input.CatalogRows = append(input.CatalogRows, identity.IdentityKey(row.Model, row.Reasoning))
	}

	known := make(map[string]struct{})
	for _, id := range usage.IDs() {
		desc, err := usage.Get(id)
		if err != nil {
			continue
		}
		known[id] = struct{}{}
		slug := routing.CatalogueSlugFor(id)
		input.Providers = append(input.Providers, routing.ProviderInput{
			Provider:   id,
			Kind:       desc.Kind,
			Windows:    desc.Windows,
			ModelsDev:  bySlug[slug],
			LiveModels: liveModels[id],
		})
	}
	if p.s.cfg != nil {
		for id := range p.s.cfg.Providers {
			if _, ok := known[id]; ok {
				continue
			}
			entries := bySlug[routing.CatalogueSlugFor(id)]
			if len(entries) == 0 && len(liveModels[id]) == 0 {
				continue
			}
			input.Providers = append(input.Providers, routing.ProviderInput{
				Provider:   id,
				Kind:       usage.KindSubscription,
				ModelsDev:  entries,
				LiveModels: liveModels[id],
			})
		}
		enabled, _ := toggle.ResolveUsageEnabled(false, p.s.cfg)
		input.Degraded = !enabled
	}

	for _, r := range p.s.routes.Routes {
		if r.Provenance != routing.ProvenanceUserDeclared {
			continue
		}
		input.Providers = append(input.Providers, routing.ProviderInput{
			Provider: r.Provider,
			UserDeclared: []routing.UserDeclaredRoute{{
				Provider:  r.Provider,
				ModelID:   r.ModelID,
				Model:     r.Model,
				Reasoning: r.Reasoning,
				WindowIDs: r.WindowIDs,
			}},
		})
	}
	return input
}
func (p *ProviderService) liveModelProviderIDsLocked() []string {
	if p.s.cfg == nil {
		return nil
	}
	enabled, _ := toggle.ResolveUsageEnabled(false, p.s.cfg)
	if !enabled {
		return nil
	}
	ids := make([]string, 0, 3)
	for _, id := range []string{"antigravity", "cursor", "codex"} {
		provider, ok := p.s.cfg.Providers[id]
		if ok && provider.Enabled {
			ids = append(ids, id)
		}
	}
	return ids
}
