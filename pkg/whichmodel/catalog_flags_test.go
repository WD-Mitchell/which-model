package whichmodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func TestStageSet(t *testing.T) {
	t.Run("case 1: single-flag stages", func(t *testing.T) {
		if got := stageSet(&GlobalFlags{RefreshBenchmarks: true}, nil); len(got) != 1 || got[0] != StageCollect {
			t.Errorf("stageSet(RefreshBenchmarks) = %v, want [StageCollect]", got)
		}
		if got := stageSet(&GlobalFlags{RefreshScores: true}, nil); len(got) != 1 || got[0] != StageDerive {
			t.Errorf("stageSet(RefreshScores) = %v, want [StageDerive]", got)
		}
		if got := stageSet(&GlobalFlags{RefreshUsage: true}, nil); len(got) != 0 {
			t.Errorf("stageSet(RefreshUsage) = %v, want empty", got)
		}
	})

	t.Run("case 2: refresh both", func(t *testing.T) {
		g := &GlobalFlags{Refresh: true}
		got := stageSet(g, nil)
		if len(got) != 2 || got[0] != StageCollect || got[1] != StageDerive {
			t.Errorf("stageSet(Refresh) = %v, want [StageCollect, StageDerive]", got)
		}
	})

	t.Run("case 3: dedup with subcommand", func(t *testing.T) {
		got := stageSet(&GlobalFlags{RefreshScores: true}, []Stage{StageDerive})
		if len(got) != 1 || got[0] != StageDerive {
			t.Errorf("stageSet() = %v, want [StageDerive]", got)
		}
	})

	t.Run("case 4: union order", func(t *testing.T) {
		got := stageSet(&GlobalFlags{RefreshBenchmarks: true}, []Stage{StageDerive})
		if len(got) != 2 || got[0] != StageCollect || got[1] != StageDerive {
			t.Errorf("stageSet() = %v, want [StageCollect, StageDerive]", got)
		}
	})
}

func TestLoadCatalogConfig(t *testing.T) {
	t.Run("case 5: defaults", func(t *testing.T) {
		cfg := config.Default()
		cc, err := loadCatalogConfig(cfg)
		if err != nil {
			t.Fatalf("loadCatalogConfig() error = %v", err)
		}
		if cc.CacheTTL != "24h" || !cc.WarnOnStaleScores {
			t.Errorf("cc = %+v, want CacheTTL 24h, WarnOnStaleScores true", cc)
		}
		if cc.RawCSVPath != "" || cc.ScoresCSVPath != "" {
			t.Errorf("cc paths = %+v, want blank", cc)
		}
	})

	t.Run("case 6: overrides", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := "[catalog]\nraw_csv_path = \"a.csv\"\nwarn_on_stale_scores = false\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		cfg, err := config.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile() error = %v", err)
		}
		cc, err := loadCatalogConfig(cfg)
		if err != nil {
			t.Fatalf("loadCatalogConfig() error = %v", err)
		}
		if cc.RawCSVPath != "a.csv" || cc.WarnOnStaleScores {
			t.Errorf("cc = %+v, want RawCSVPath a.csv, WarnOnStaleScores false", cc)
		}
	})

	t.Run("case 7: unknown key", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := "[catalog]\nbogus = 1\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		cfg, err := config.LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile() error = %v", err)
		}
		if _, err := loadCatalogConfig(cfg); err == nil {
			t.Error("loadCatalogConfig() error = nil, want error for unknown key")
		}
	})
}

func TestResolveCatalogPaths(t *testing.T) {
	t.Run("case 8: path defaults", func(t *testing.T) {
		res := resolveCatalogPaths(DefaultCatalogConfig(), config.Paths{CacheDir: "/tmp/x"}, "")
		if res.RawCSVPath != filepath.Join("/tmp/x", "catalog", "available_model_raw_values.csv") {
			t.Errorf("RawCSVPath = %q", res.RawCSVPath)
		}
		if res.ScoresCSVPath != filepath.Join("/tmp/x", "catalog", "available_model_scores.csv") {
			t.Errorf("ScoresCSVPath = %q", res.ScoresCSVPath)
		}
		if res.CatalogueCachePath != filepath.Join("/tmp/x", "catalog", "modelsdev_providers.json") {
			t.Errorf("CatalogueCachePath = %q", res.CatalogueCachePath)
		}
	})

	t.Run("case 9: walk-up", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		providersPath := filepath.Join(root, "providers.toml")
		if err := os.WriteFile(providersPath, []byte(""), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		cwd := filepath.Join(root, "sub")
		res := resolveCatalogPaths(DefaultCatalogConfig(), config.Paths{CacheDir: "/tmp/x"}, cwd)
		if res.ProviderConfigPath != providersPath {
			t.Errorf("ProviderConfigPath = %q, want %q", res.ProviderConfigPath, providersPath)
		}
		if err := os.Remove(providersPath); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		res2 := resolveCatalogPaths(DefaultCatalogConfig(), config.Paths{CacheDir: "/tmp/x"}, cwd)
		if res2.ProviderConfigPath != "" {
			t.Errorf("ProviderConfigPath = %q, want empty", res2.ProviderConfigPath)
		}
		if got := findRepoRoot(cwd); got != root {
			t.Errorf("findRepoRoot() = %q, want %q", got, root)
		}
	})
}

func TestParseCacheTTL(t *testing.T) {
	t.Run("case 10", func(t *testing.T) {
		d, err := parseCacheTTL("24h")
		if err != nil {
			t.Fatalf("parseCacheTTL() error = %v", err)
		}
		if d.Hours() != 24 {
			t.Errorf("d = %v, want 24h", d)
		}
		if _, err := parseCacheTTL("nope"); err == nil {
			t.Error("parseCacheTTL(nope) error = nil, want error")
		}
	})
}

func TestValidateAdd(t *testing.T) {
	t.Run("case 11", func(t *testing.T) {
		if err := validateAdd([]string{"aa_page"}); err != nil {
			t.Errorf("validateAdd([aa_page]) error = %v, want nil", err)
		}
		var ue *UsageError
		err := validateAdd([]string{"nope"})
		if err == nil {
			t.Fatal("validateAdd([nope]) error = nil, want error")
		}
		if !isUsageError(err, &ue) {
			t.Errorf("validateAdd(nope) error = %T, want *UsageError", err)
		}
	})
}

func TestValidateWorkflowFlags(t *testing.T) {
	t.Run("case 12", func(t *testing.T) {
		err := validateWorkflowFlags(&catalogFlags{Write: "a", Check: "b"})
		if err == nil {
			t.Fatal("validateWorkflowFlags() error = nil, want error")
		}
		if err := validateWorkflowFlags(&catalogFlags{Write: "a"}); err != nil {
			t.Errorf("validateWorkflowFlags() error = %v, want nil", err)
		}
	})
}

func isUsageError(err error, target **UsageError) bool {
	ue, ok := err.(*UsageError)
	if ok {
		*target = ue
	}
	return ok
}
