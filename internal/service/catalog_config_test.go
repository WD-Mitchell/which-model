package service

import (
	"github.com/WD-Mitchell/which-model/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestBenchmarkPathWithPublishingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[catalog]\nbenchmark_config_path = 'custom/benchmarks.toml'\n[catalog.publish]\nrun_tests = true\nenabled = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	svc := Services{cfg: cfg}
	if got := svc.catalogBenchmarkPath(); got != "custom/benchmarks.toml" {
		t.Fatalf("path = %q", got)
	}
}
