package whichmodel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/WD-Mitchell/which-model/internal/config"
)

// Stage is one catalog pipeline stage (specs/features/F23-cmd-catalog/CONTRACTS.md).
type Stage int

const (
	StageCollect Stage = iota
	StageDerive
)

// CatalogConfig is the F23-owned [catalog] config section (F01 DECISION B).
type CatalogConfig struct {
	RawCSVPath          string `toml:"raw_csv_path"`
	ScoresCSVPath       string `toml:"scores_csv_path"`
	ProviderConfigPath  string `toml:"provider_config_path"`
	BenchmarkConfigPath string `toml:"benchmark_config_path"`
	CacheTTL            string `toml:"cache_ttl"`
	WarnOnStaleScores   bool   `toml:"warn_on_stale_scores"`
}

// DefaultCatalogConfig returns the [catalog] defaults.
func DefaultCatalogConfig() CatalogConfig {
	return CatalogConfig{
		CacheTTL:          "24h",
		WarnOnStaleScores: true,
	}
}

// ResolvedCatalog is CatalogConfig with every path defaulted and resolved to
// an absolute or repo-relative location.
type ResolvedCatalog struct {
	RawCSVPath          string
	ScoresCSVPath       string
	ProviderConfigPath  string
	BenchmarkConfigPath string
	CatalogueCachePath  string
}

// loadCatalogConfig decodes the [catalog] section into CatalogConfig's
// defaults; unknown keys are rejected (F01's strict UnmarshalKey).
func loadCatalogConfig(cfg *config.Config) (CatalogConfig, error) {
	c := DefaultCatalogConfig()
	err := cfg.UnmarshalKey("catalog", &c)
	return c, err
}

// resolveCatalogPaths fills every blank ResolvedCatalog path from paths and
// walking cwd upward for providers.toml/benchmarks.toml.
func resolveCatalogPaths(c CatalogConfig, paths config.Paths, cwd string) ResolvedCatalog {
	raw := c.RawCSVPath
	if raw == "" {
		raw = filepath.Join(paths.CacheDir, "catalog", "available_model_raw_values.csv")
	}
	scores := c.ScoresCSVPath
	if scores == "" {
		scores = filepath.Join(paths.CacheDir, "catalog", "available_model_scores.csv")
	}
	cache := filepath.Join(paths.CacheDir, "catalog", "modelsdev_providers.json")
	provider := c.ProviderConfigPath
	if provider == "" {
		provider = walkUp(cwd, "providers.toml")
	}
	benchmark := c.BenchmarkConfigPath
	if benchmark == "" {
		benchmark = walkUp(cwd, "benchmarks.toml")
	}
	return ResolvedCatalog{
		RawCSVPath:          raw,
		ScoresCSVPath:       scores,
		ProviderConfigPath:  provider,
		BenchmarkConfigPath: benchmark,
		CatalogueCachePath:  cache,
	}
}

// isGitBoundary reports whether dir contains a .git entry (directory or file).
func isGitBoundary(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// walkUp walks start upward looking for name, stopping after checking the
// first directory containing a .git boundary. If no .git boundary is ever
// found, it walks all the way to the filesystem root.
func walkUp(start, name string) string {
	if start == "" {
		return ""
	}
	dir := start
	for {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		if isGitBoundary(dir) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// findRepoRoot returns the first directory from cwd upward containing a
// .git boundary, or "" if none.
func findRepoRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	dir := cwd
	for {
		if isGitBoundary(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// parseCacheTTL wraps time.ParseDuration, mapping failures to a ConfigError.
func parseCacheTTL(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, &config.ConfigError{Kind: config.KindInvalidValue, Key: "catalog.cache_ttl", Err: err}
	}
	return d, nil
}

// providerTOML mirrors providers.toml's shape for strict decoding.
type providerTOML struct {
	Providers map[string]struct {
		ExcludedModels []string `toml:"excluded_models"`
	} `toml:"providers"`
}

// loadProviderConfig strictly decodes providers.toml, rejecting blank ids,
// blank/duplicate excluded entries, and unknown keys.
func loadProviderConfig(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("provider config not found at %s: %w", path, err)
	}
	var doc providerTOML
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, &config.ConfigError{Kind: config.KindInvalidValue, Path: path, Err: err}
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, &config.ConfigError{Kind: config.KindInvalidValue, Path: path, Err: fmt.Errorf("unknown key %q", undecoded[0].String())}
	}
	out := make(map[string][]string, len(doc.Providers))
	for id, p := range doc.Providers {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("provider config %s: blank provider id", path)
		}
		seen := make(map[string]bool, len(p.ExcludedModels))
		for _, m := range p.ExcludedModels {
			if strings.TrimSpace(m) == "" {
				return nil, fmt.Errorf("provider config %s: provider %q has a blank excluded_models entry", path, id)
			}
			if seen[m] {
				return nil, fmt.Errorf("provider config %s: provider %q has a duplicate excluded_models entry %q", path, id, m)
			}
			seen[m] = true
		}
		out[id] = p.ExcludedModels
	}
	return out, nil
}

// benchmarkSelectionTOML mirrors benchmarks.toml's shape for strict decoding.
type benchmarkSelectionTOML struct {
	Selection struct {
		Groups     []string `toml:"groups"`
		Benchmarks []string `toml:"benchmarks"`
	} `toml:"benchmark_selection"`
	Groups map[string]struct {
		Benchmarks []string `toml:"benchmarks"`
	} `toml:"benchmark_groups"`
}

// validateNameList rejects blank or duplicate entries within one list.
func validateNameList(path, context string, names []string) error {
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if strings.TrimSpace(n) == "" {
			return fmt.Errorf("benchmarks config %s: %s has a blank entry", path, context)
		}
		if seen[n] {
			return fmt.Errorf("benchmarks config %s: %s has a duplicate entry %q", path, context, n)
		}
		seen[n] = true
	}
	return nil
}

// loadBenchmarkConfig strictly decodes benchmarks.toml and returns the
// expanded benchmark name list: group lists in declared order, then the
// direct list, deduplicated keeping first occurrence.
func loadBenchmarkConfig(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("benchmarks config not found at %s: %w", path, err)
	}
	var doc benchmarkSelectionTOML
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, &config.ConfigError{Kind: config.KindInvalidValue, Path: path, Err: err}
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, &config.ConfigError{Kind: config.KindInvalidValue, Path: path, Err: fmt.Errorf("unknown key %q", undecoded[0].String())}
	}
	if err := validateNameList(path, "benchmark_selection.groups", doc.Selection.Groups); err != nil {
		return nil, err
	}
	if err := validateNameList(path, "benchmark_selection.benchmarks", doc.Selection.Benchmarks); err != nil {
		return nil, err
	}
	for _, g := range doc.Selection.Groups {
		group, ok := doc.Groups[g]
		if !ok {
			return nil, fmt.Errorf("benchmarks config %s: group %q has no [benchmark_groups.%s] table", path, g, g)
		}
		if err := validateNameList(path, fmt.Sprintf("benchmark_groups.%s.benchmarks", g), group.Benchmarks); err != nil {
			return nil, err
		}
	}

	seen := make(map[string]bool)
	var expanded []string
	for _, g := range doc.Selection.Groups {
		for _, name := range doc.Groups[g].Benchmarks {
			if !seen[name] {
				seen[name] = true
				expanded = append(expanded, name)
			}
		}
	}
	for _, name := range doc.Selection.Benchmarks {
		if !seen[name] {
			seen[name] = true
			expanded = append(expanded, name)
		}
	}
	return expanded, nil
}
