package whichmodel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
)

func TestCatalogRebuildBypassesCachedModelsAndOldRawData(t *testing.T) {
	installInsecureTransport(t)
	srv := newModelsDevTestServer(t, `[{"provider":"openai","id":"gpt-6-astra","name":"GPT-6-Astra"}]`, "[]")
	defer srv.Close()
	withModelsDevSeams(t, srv)
	aaSrv := newAATestServer(t, `{"pagination":{"page":1,"has_more":false},"data":[]}`)
	defer aaSrv.Close()
	withAASeam(t, aaSrv)
	dir := t.TempDir()
	bench := filepath.Join(dir, "benchmarks.toml")
	raw := filepath.Join(dir, "raw.csv")
	cache := filepath.Join(dir, "models.json")
	if err := os.WriteFile(bench, []byte("[benchmark_selection]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(raw, []byte("corrupted old raw data\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeCache(cache, []modelsdev.ProviderModel{{Provider: "openai", ModelID: "old", Name: "Old Model"}}); err != nil {
		t.Fatal(err)
	}
	_, err := (defaultRunner{}).Collect(context.Background(), CollectOptions{BenchmarksPath: bench, OutPath: raw, CacheTTL: time.Hour, CatalogueCachePath: cache, Timeout: 2 * time.Second, AAKey: "test-key", Rebuild: true})
	if err != nil {
		t.Fatal(err)
	}
	if srv.apiCount != 1 {
		t.Fatalf("model catalog fetches=%d, want 1 despite fresh cache", srv.apiCount)
	}
	rows, _, err := csvstore.Read(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Values[0] != "GPT-6-Astra" {
		t.Fatalf("rebuilt raw rows=%v", rows)
	}
}

func TestCatalogRefreshRebuildFlagReachesCollector(t *testing.T) {
	t.Setenv("ARTIFICIAL_ANALYSIS_API", "test-key")
	previous := newRunner
	t.Cleanup(func() { newRunner = previous })
	fake := &fakeStageRunner{}
	newRunner = func() StageRunner { return fake }
	dir := t.TempDir()
	code, _, errOut := captureExecuteFresh(t, []string{"catalog", "refresh", "--rebuild", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errOut)
	}
	if !fake.capturedCO.Rebuild {
		t.Fatal("refresh did not request a full rebuild")
	}
}
