package whichmodel

import (
	"github.com/WD-Mitchell/which-model/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogConfigAcceptsPublishingAndConfiguredScores(t *testing.T) {
	for _, body := range []string{`[catalog]
scores_csv_path = "custom scores.csv"
raw_csv_path = "raw.csv"
[catalog.publish]
enabled = false
`, `[catalog]
scores_csv_path = "custom scores.csv"
[catalog.publish]
enabled = true
branches = ["main"]
`} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loadCatalogConfig(cfg); err != nil {
			t.Fatal(err)
		}
		got, err := scoresCSVPath(cfg)
		if err != nil || got != "custom scores.csv" {
			t.Fatalf("scores path = %q %v", got, err)
		}
	}
}

func TestCatalogConfigRejectsUnknownKeys(t *testing.T) {
	for _, section := range []string{"catalog", "catalog.publish"} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte("["+section+"]\nunknown=true\n"), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := loadCatalogConfig(cfg); err == nil {
			t.Fatalf("%s: wanted unknown key rejection", section)
		}
	}
}

func TestScoresPathEnvOnly(t *testing.T) {
	cfg := config.Default()
	env := []string{"WHICH_MODEL_CATALOG_SCORES_CSV_PATH=env.csv"}
	if err := config.ApplyEnv(cfg, func(string) string { return "env.csv" }, env); err != nil {
		t.Fatal(err)
	}
	got, err := scoresCSVPath(cfg)
	if err != nil || got != "env.csv" {
		t.Fatalf("scores path = %q %v", got, err)
	}
}
