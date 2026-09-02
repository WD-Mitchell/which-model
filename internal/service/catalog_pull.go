package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

const (
	catalogRepoScoresRel     = "data/available_model_scores.csv"
	catalogRepoRawRel        = "data/available_model_raw_values.csv"
	catalogRepoBenchmarksRel = "config/benchmarks.toml"
	aaKeyFileName            = "aa_api_key"
	aaKeyClearSentinel       = "-"
)

// fetchCatalogRepoFile is the network seam for GitHub raw downloads. Tests
// replace it so RefreshRoutes never hits the network.
var fetchCatalogRepoFile = fetchCatalogRepoFileLive

func fetchCatalogRepoFileLive(ctx context.Context, rawURL string) ([]byte, error) {
	client := httpkit.NewClient(httpkit.WithTimeout(30*time.Second), httpkit.WithMaxBytes(2<<20))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(ctx, req)
}

func catalogRepoRawURL(owner, repo, ref, rel string) string {
	return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + ref + "/" + rel
}

func (s *Services) refreshCatalogSource(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	gui, guiErr := s.cfg.LoadGUI()
	key := readAAKeyFile(s.paths.ConfigDir)
	s.mu.RUnlock()
	if guiErr != nil {
		gui = config.DefaultGUIConfig()
	}

	if gui.UseLocalAA {
		if s.catalogRefresh == nil {
			return fmt.Errorf("local Artificial Analysis collect is not available in this build")
		}
		if key != "" {
			prev, had := os.LookupEnv("ARTIFICIAL_ANALYSIS_API")
			_ = os.Setenv("ARTIFICIAL_ANALYSIS_API", key)
			defer func() {
				if had {
					_ = os.Setenv("ARTIFICIAL_ANALYSIS_API", prev)
				} else {
					_ = os.Unsetenv("ARTIFICIAL_ANALYSIS_API")
				}
			}()
		}
		return s.catalogRefresh(ctx)
	}
	return s.pullCatalogFromRepo(ctx, gui.CatalogRepo)
}

func (s *Services) pullCatalogFromRepo(ctx context.Context, spec string) error {
	owner, repo, ref, err := config.ParseCatalogRepoSpec(spec)
	if err != nil {
		return err
	}
	type item struct {
		rel  string
		dest string
		kind string
	}
	dir := filepath.Join(s.paths.CacheDir, "catalog")
	items := []item{
		{catalogRepoScoresRel, filepath.Join(dir, "available_model_scores.csv"), "scores"},
		{catalogRepoRawRel, filepath.Join(dir, "available_model_raw_values.csv"), "raw"},
		{catalogRepoBenchmarksRel, filepath.Join(dir, "benchmarks.toml"), "benchmarks"},
	}
	bodies := make([][]byte, len(items))
	for i, it := range items {
		rawURL := catalogRepoRawURL(owner, repo, ref, it.rel)
		body, err := fetchCatalogRepoFile(ctx, rawURL)
		if err != nil {
			return fmt.Errorf("catalog from %s/%s@%s: %s: %w", owner, repo, ref, it.rel, err)
		}
		if err := validateCatalogRepoFile(it.kind, body); err != nil {
			return fmt.Errorf("catalog from %s/%s@%s: %s: %w", owner, repo, ref, it.rel, err)
		}
		bodies[i] = body
	}
	for i, it := range items {
		if err := config.AtomicWriteFile(it.dest, bodies[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateCatalogRepoFile(kind string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("empty file")
	}
	text := string(body)
	switch kind {
	case "scores", "raw":
		first, _, _ := strings.Cut(text, "\n")
		first = strings.TrimPrefix(first, "\ufeff")
		if strings.HasPrefix(first, "#") {
			_, rest, ok := strings.Cut(text, "\n")
			if !ok {
				return fmt.Errorf("missing CSV header")
			}
			first, _, _ = strings.Cut(rest, "\n")
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(first)), "model,") {
			return fmt.Errorf("not a scores/raw CSV (header %q)", strings.TrimSpace(first))
		}
	case "benchmarks":
		if !strings.Contains(text, "[") {
			return fmt.Errorf("not a benchmarks.toml")
		}
	}
	return nil
}

func aaKeyPath(configDir string) string {
	return filepath.Join(configDir, aaKeyFileName)
}

func readAAKeyFile(configDir string) string {
	data, err := os.ReadFile(aaKeyPath(configDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func writeAAKeyFile(configDir, key string) error {
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(aaKeyPath(configDir), []byte(strings.TrimSpace(key)+"\n"), 0o600)
}

func clearAAKeyFile(configDir string) error {
	err := os.Remove(aaKeyPath(configDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
