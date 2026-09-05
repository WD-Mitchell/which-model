package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// Provider inventories are independent of benchmark coverage. The route table
// only contains scored matches, so it cannot be the store for new releases.
func providerInventoryPath(cacheDir string) string {
	return filepath.Join(cacheDir, "catalog", "provider_models.json")
}

func readProviderInventory(cacheDir string) map[string][]routing.ModelEntry {
	doc := harnessDocument(providerInventoryPath(cacheDir))
	data, _ := json.Marshal(doc)
	var inventory map[string][]routing.ModelEntry
	_ = json.Unmarshal(data, &inventory)
	if inventory == nil {
		inventory = map[string][]routing.ModelEntry{}
	}
	return inventory
}

func writeProviderInventory(cacheDir string, fresh map[string][]routing.ModelEntry) error {
	inventory := readProviderInventory(cacheDir)
	for id, models := range fresh {
		inventory[id] = models
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		return err
	}
	return config.AtomicWriteFile(providerInventoryPath(cacheDir), data)
}

// Codex already refreshes this non-secret model metadata for the signed-in
// account. Read it on demand so a release appears on the next Models read,
// without waiting for benchmarks or another network refresh. No auth file is read.
func (s *Services) codexModels() []routing.ModelEntry {
	home := s.harnessHome
	var root string
	if home == "" {
		home, _ = os.UserHomeDir()
		root = os.Getenv("CODEX_HOME")
	}
	if root == "" {
		root = filepath.Join(home, ".codex")
	}
	doc := harnessDocument(filepath.Join(root, "models_cache.json"))
	rows, ok := doc["models"].([]any)
	if !ok {
		return nil
	}
	models := make([]routing.ModelEntry, 0, len(rows))
	seen := map[string]bool{}
	for _, value := range rows {
		row := object(value)
		id := textValue(row["slug"])
		if !providerModelIDPattern.MatchString(id) || seen[id] || textValue(row["visibility"]) != "list" {
			continue
		}
		seen[id] = true
		name := textValue(row["display_name"])
		if name == "" {
			name = id
		}
		model := routing.ModelEntry{ModelID: id, Name: identity.CleanModelName(name)}
		levels := map[string]bool{}
		if options, ok := row["supported_reasoning_levels"].([]any); ok {
			for _, option := range options {
				level, valid := identity.ParseEffort(textValue(object(option)["effort"]))
				if valid && !levels[level] {
					model.Reasoning = append(model.Reasoning, level)
					levels[level] = true
				}
			}
		}
		models = append(models, model)
	}
	return models
}

func (s *Services) providerInventoryLocked(id string) []routing.ModelEntry {
	if !s.cfg.Providers[id].Enabled {
		return nil
	}
	if id == "codex" {
		if models := s.codexModels(); models != nil {
			return models
		}
	}
	return readProviderInventory(s.paths.CacheDir)[id]
}

func inventoryModelName(model routing.ModelEntry) string {
	if name := identity.CleanModelName(model.Name); strings.TrimSpace(name) != "" {
		return name
	}
	return model.ModelID
}
