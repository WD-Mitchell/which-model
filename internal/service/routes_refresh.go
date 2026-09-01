package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

const modelsDevCacheFile = "modelsdev_providers.json"

// fetchModelsDevCatalogue is the network seam for RefreshRoutes. Tests replace
// it so a missing cache never hits models.dev.
var fetchModelsDevCatalogue = fetchModelsDevCatalogueLive

func fetchModelsDevCatalogueLive() ([]modelsdev.ProviderModel, error) {
	// models.dev api.json is ~4 MB (F08); the default 256 KiB bound is too small.
	client := httpkit.NewClient(httpkit.WithTimeout(30*time.Second), httpkit.WithMaxBytes(16<<20))
	return modelsdev.FetchModelsDevProvidersFrom(client, modelsdev.ProvidersURL)
}

func modelsDevCachePath(cacheDir string) string {
	return filepath.Join(cacheDir, "catalog", modelsDevCacheFile)
}

// RefreshRoutes rebuilds the route table the same way `which-model routes
// refresh` does: models.dev catalogue (cache, else fetch) joined to the
// scores CSV, preserving user-declared routes. Settings calls this after a
// successful sign-in and from the signed-in "Refresh models" button.
func (p *ProviderService) RefreshRoutes(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	catalogue, err := p.loadOrFetchModelsDev()
	if err != nil {
		return toErrorDTO(fmt.Errorf("refresh models: %w", err))
	}
	p.s.mu.RLock()
	input := p.routeProductionInputLocked(catalogue)
	existing := p.s.routes
	p.s.mu.RUnlock()

	result, err := routing.ProduceRoutes(input)
	if err != nil {
		return toErrorDTO(err)
	}
	routes := result.Routes
	if routes == nil {
		routes = []routing.Route{}
	}
	table := routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		ScoresHash:    existing.ScoresHash,
		RefreshedAt:   existing.RefreshedAt,
		Routes:        routes,
	}

	p.s.mu.Lock()
	if err := routing.SaveTable(filepath.Join(p.s.paths.CacheDir, "routes.json"), table); err != nil {
		p.s.mu.Unlock()
		return toErrorDTO(err)
	}
	p.s.routes = table
	p.s.mu.Unlock()
	p.s.emit(EventConfigChanged, map[string]string{"section": "routes"})
	return nil
}

func (p *ProviderService) loadOrFetchModelsDev() ([]modelsdev.ProviderModel, error) {
	path := modelsDevCachePath(p.s.paths.CacheDir)
	if cached, ok := readModelsDevCache(path); ok {
		return cached, nil
	}
	catalogue, err := fetchModelsDevCatalogue()
	if err != nil {
		return nil, err
	}
	if err := writeModelsDevCache(path, catalogue); err != nil {
		// Routes can still be built from the in-memory catalogue.
		return catalogue, nil
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
	return os.WriteFile(path, data, 0o644)
}

func (p *ProviderService) routeProductionInputLocked(catalogue []modelsdev.ProviderModel) routing.Input {
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
			Provider:  id,
			Kind:      desc.Kind,
			Windows:   desc.Windows,
			ModelsDev: bySlug[slug],
		})
	}
	if p.s.cfg != nil {
		for id := range p.s.cfg.Providers {
			if _, ok := known[id]; ok {
				continue
			}
			entries := bySlug[routing.CatalogueSlugFor(id)]
			if len(entries) == 0 {
				continue
			}
			input.Providers = append(input.Providers, routing.ProviderInput{
				Provider:  id,
				Kind:      usage.KindSubscription,
				ModelsDev: entries,
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
