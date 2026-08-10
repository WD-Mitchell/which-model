package whichmodel

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/aa"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
	sdecimal "github.com/shopspring/decimal"
)

// installInsecureTransport trusts httptest.NewTLSServer's self-signed cert
// for the duration of one test (matches F08's own test pattern: httpkit has
// no test-mode escape hatch, so http.DefaultTransport is patched directly).
func installInsecureTransport(t *testing.T) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only
	}
	t.Cleanup(func() { http.DefaultTransport = old })
}

func dec(s string) sdecimal.Decimal {
	d, err := sdecimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func decPtr(s string) *sdecimal.Decimal {
	d := dec(s)
	return &d
}

func TestBuildFreshRows(t *testing.T) {
	t.Run("case 1: happy merge", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5", EffortLevels: []string{"low", "high"}},
		}
		aaModels := []aa.AAModel{
			{Slug: "opus5", IntelligenceIndex: decPtr("70"), CodingIndex: decPtr("80"), AgenticIndex: decPtr("90"), MedianResponseSeconds: decPtr("1.5"), CostPerTaskUSD: decPtr("0.5")},
		}
		rows, err := buildFreshRows(catalogue, nil, aaModels, nil, nil)
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("len(rows) = %d, want 2", len(rows))
		}
		for _, r := range rows {
			if r.Header[0] != "model" || len(r.Header) != len(csvstore.RawCoreColumns) {
				t.Errorf("header = %v", r.Header)
			}
			if r.Values[2] != "70" {
				t.Errorf("intelligence_index = %q, want 70", r.Values[2])
			}
		}
	})

	t.Run("case 2: AA unmatched dropped", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5"},
		}
		aaModels := []aa.AAModel{
			{Slug: "unrelated-slug", IntelligenceIndex: decPtr("70")},
		}
		rows, err := buildFreshRows(catalogue, nil, aaModels, nil, nil)
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		if len(rows) != 1 || rows[0].Values[2] != "" {
			t.Errorf("rows = %+v, want blank intelligence_index (AA item unmatched)", rows)
		}
	})

	t.Run("case 3: catalogue unmatched blank cells", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5"},
		}
		rows, err := buildFreshRows(catalogue, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		if len(rows) != 1 || rows[0].Values[2] != "" {
			t.Errorf("rows = %+v, want blank metric cells", rows)
		}
	})

	t.Run("case 4: benchmark priority AA wins", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5"},
		}
		aaModels := []aa.AAModel{
			{Slug: "opus5", Benchmarks: map[string]sdecimal.Decimal{"benchmark:SciCode": dec("55")}},
		}
		benchmarks := []modelsdev.BenchmarkRecord{
			{CanonicalID: "anthropic/opus5", Benchmarks: []modelsdev.BenchmarkEvidence{{Name: "SciCode", Score: dec("10")}}},
		}
		rows, err := buildFreshRows(catalogue, benchmarks, aaModels, nil, []string{"SciCode"})
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		idx := colIndex(rows[0].Header, "benchmark:SciCode")
		if rows[0].Values[idx] != "55" {
			t.Errorf("SciCode cell = %q, want 55 (AA wins)", rows[0].Values[idx])
		}
	})

	t.Run("case 5: effort scoped evidence", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5", EffortLevels: []string{"low", "high"}},
		}
		benchmarks := []modelsdev.BenchmarkRecord{
			{CanonicalID: "anthropic/opus5", Benchmarks: []modelsdev.BenchmarkEvidence{{Name: "SciCode", Score: dec("42"), Effort: "high"}}},
		}
		rows, err := buildFreshRows(catalogue, benchmarks, nil, nil, []string{"SciCode"})
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		idx := colIndex(rows[0].Header, "benchmark:SciCode")
		var lowVal, highVal string
		for _, r := range rows {
			if r.Values[1] == "low" {
				lowVal = r.Values[idx]
			}
			if r.Values[1] == "high" {
				highVal = r.Values[idx]
			}
		}
		if lowVal != "" {
			t.Errorf("low row SciCode = %q, want blank", lowVal)
		}
		if highVal != "42" {
			t.Errorf("high row SciCode = %q, want 42", highVal)
		}
	})

	t.Run("case 6: expansion scoping excludes non-selected name", func(t *testing.T) {
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "opus5", Name: "Claude Opus 5"},
		}
		benchmarks := []modelsdev.BenchmarkRecord{
			{CanonicalID: "anthropic/opus5", Benchmarks: []modelsdev.BenchmarkEvidence{{Name: "Excluded", Score: dec("1")}}},
		}
		rows, err := buildFreshRows(catalogue, benchmarks, nil, nil, nil)
		if err != nil {
			t.Fatalf("buildFreshRows() error = %v", err)
		}
		if colIndex(rows[0].Header, "benchmark:Excluded") != -1 {
			t.Error("Excluded column present, want absent (not in expandedNames)")
		}
	})
}

func colIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

func TestMergeWithExisting(t *testing.T) {
	header := []string{"model", "reasoning", "intelligence_index", "time_per_intelligence_index_task_seconds", "cost_per_intelligence_index_task_usd", "median_end_to_end_response_time_seconds", "artificial_analysis_coding_index", "artificial_analysis_agentic_index"}

	t.Run("case 7: merge preserves existing non-blank cell", func(t *testing.T) {
		existing := []csvstore.Row{{Header: header, Values: []string{"M", "high", "50", "", "", "", "", ""}}}
		fresh := []csvstore.Row{{Header: header, Values: []string{"M", "high", "", "", "", "", "", ""}}}
		got, err := mergeWithExisting(existing, fresh, nil)
		if err != nil {
			t.Fatalf("mergeWithExisting() error = %v", err)
		}
		if len(got) != 1 || got[0].Values[2] != "50" {
			t.Errorf("got = %+v, want existing value 50 preserved", got)
		}
	})

	t.Run("case 8: subset preserve via MergePartialRefresh", func(t *testing.T) {
		existing := []csvstore.Row{
			{Header: header, Values: []string{"Kept", "high", "10", "", "", "", "", ""}},
			{Header: header, Values: []string{"M", "high", "50", "", "", "", "", ""}},
		}
		fresh := []csvstore.Row{{Header: header, Values: []string{"M", "high", "60", "", "", "", "", ""}}}
		got, err := mergeWithExisting(existing, fresh, []string{"M"})
		if err != nil {
			t.Fatalf("mergeWithExisting() error = %v", err)
		}
		found := false
		for _, r := range got {
			if r.Values[0] == "Kept" {
				found = true
			}
		}
		if !found {
			t.Errorf("got = %+v, want unselected model 'Kept' preserved", got)
		}
	})
}

// modelsDevTestServer serves api.json and models.json at the pinned paths,
// counting requests to each.
type modelsDevTestServer struct {
	*httptest.Server
	apiCount, modelsCount int
}

func newModelsDevTestServer(t *testing.T, apiJSON, modelsJSON string) *modelsDevTestServer {
	t.Helper()
	s := &modelsDevTestServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api.json", func(w http.ResponseWriter, r *http.Request) {
		s.apiCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(apiJSON))
	})
	mux.HandleFunc("/models.json", func(w http.ResponseWriter, r *http.Request) {
		s.modelsCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(modelsJSON))
	})
	s.Server = httptest.NewTLSServer(mux)
	return s
}

// withModelsDevSeams points the modelsDevProvidersURL/modelsDevBenchmarksURL
// seams at srv, restoring the real F08 constants on cleanup.
func withModelsDevSeams(t *testing.T, srv *modelsDevTestServer) {
	t.Helper()
	origProviders, origBenchmarks := modelsDevProvidersURL, modelsDevBenchmarksURL
	modelsDevProvidersURL = srv.URL + "/api.json"
	modelsDevBenchmarksURL = srv.URL + "/models.json"
	t.Cleanup(func() {
		modelsDevProvidersURL, modelsDevBenchmarksURL = origProviders, origBenchmarks
	})
}

// aaTestServer serves an AA v2 envelope with request counting.
type aaTestServer struct {
	*httptest.Server
	count int
}

func newAATestServer(t *testing.T, envelope string) *aaTestServer {
	t.Helper()
	s := &aaTestServer{}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.count++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(envelope))
	}))
	return s
}

// withAASeam points aaV2Fetch/aaPageFetch at srv, restoring the real F08
// wrappers on cleanup.
func withAASeam(t *testing.T, srv *aaTestServer) {
	t.Helper()
	origV2, origPage := aaV2Fetch, aaPageFetch
	aaV2Fetch = func(client *httpkit.Client, apiKey string) ([]aa.AAModel, error) {
		return aa.FetchAAv2From(client, apiKey, srv.URL, srv.URL)
	}
	aaPageFetch = func(client *httpkit.Client, slug string, requireFallbackCost bool) (*aa.PageMetrics, error) {
		return nil, nil
	}
	t.Cleanup(func() {
		aaV2Fetch, aaPageFetch = origV2, origPage
	})
}

func TestCatalogueCache(t *testing.T) {
	t.Run("case 9: cache reuse within ttl, refetch after expiry", func(t *testing.T) {
		dir := t.TempDir()
		cachePath := filepath.Join(dir, "cache.json")
		catalogue := []modelsdev.ProviderModel{{Provider: "p", ModelID: "m", Name: "M"}}
		if err := writeCache(cachePath, catalogue); err != nil {
			t.Fatalf("writeCache() error = %v", err)
		}
		if !cacheFresh(cachePath, time.Hour) {
			t.Error("cacheFresh() = false, want true (just written)")
		}
		got, ok, err := readCache(cachePath)
		if err != nil || !ok || len(got) != 1 {
			t.Errorf("readCache() = (%v, %v, %v)", got, ok, err)
		}
		old := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(cachePath, old, old); err != nil {
			t.Fatalf("Chtimes() error = %v", err)
		}
		if cacheFresh(cachePath, time.Hour) {
			t.Error("cacheFresh() = true, want false (aged past ttl)")
		}
	})

	t.Run("case 9b: readCache missing file", func(t *testing.T) {
		_, ok, err := readCache(filepath.Join(t.TempDir(), "missing.json"))
		if err != nil || ok {
			t.Errorf("readCache() = (ok=%v, err=%v), want (false, nil)", ok, err)
		}
	})

	t.Run("case 9c: Collect reuses a fresh cache across two runs (api.json fetched once)", func(t *testing.T) {
		installInsecureTransport(t)
		apiJSON := `[{"provider":"anthropic","id":"opus5","name":"Claude Opus 5"}]`
		srv := newModelsDevTestServer(t, apiJSON, "[]")
		defer srv.Close()
		withModelsDevSeams(t, srv)
		aaSrv := newAATestServer(t, `{"pagination":{"page":1,"has_more":false},"data":[]}`)
		defer aaSrv.Close()
		withAASeam(t, aaSrv)

		dir := t.TempDir()
		providersPath := filepath.Join(dir, "providers.toml")
		os.WriteFile(providersPath, []byte("[providers.anthropic]\nexcluded_models=[]\n"), 0o644)
		benchPath := filepath.Join(dir, "b.toml")
		os.WriteFile(benchPath, []byte("[benchmark_selection]\n"), 0o644)

		opts := CollectOptions{
			ProviderConfigPath: providersPath,
			BenchmarksPath:     benchPath,
			OutPath:            filepath.Join(dir, "raw.csv"),
			Timeout:            2 * time.Second,
			CacheTTL:           time.Hour,
			CatalogueCachePath: filepath.Join(dir, "cache.json"),
			AAKey:              "key",
		}
		if _, err := (defaultRunner{}).Collect(context.Background(), opts); err != nil {
			t.Fatalf("Collect() 1 error = %v", err)
		}
		if _, err := (defaultRunner{}).Collect(context.Background(), opts); err != nil {
			t.Fatalf("Collect() 2 error = %v", err)
		}
		if srv.apiCount != 1 {
			t.Errorf("apiCount = %d, want 1 (second Collect reused the cache)", srv.apiCount)
		}

		old := time.Now().Add(-2 * time.Hour)
		os.Chtimes(opts.CatalogueCachePath, old, old)
		if _, err := (defaultRunner{}).Collect(context.Background(), opts); err != nil {
			t.Fatalf("Collect() 3 error = %v", err)
		}
		if srv.apiCount != 2 {
			t.Errorf("apiCount = %d, want 2 (aged cache triggered a refetch)", srv.apiCount)
		}
	})
}

func TestDefaultRunnerCollect(t *testing.T) {
	t.Run("case 10: fail-fast ordering, no HTTP before validation", func(t *testing.T) {
		srv := newModelsDevTestServer(t, "{}", "[]")
		defer srv.Close()
		withModelsDevSeams(t, srv)
		dir := t.TempDir()
		_, err := (defaultRunner{}).Collect(context.Background(), CollectOptions{
			ProviderConfigPath: filepath.Join(dir, "missing-providers.toml"),
			BenchmarksPath:     filepath.Join(dir, "b.toml"),
			OutPath:            filepath.Join(dir, "raw.csv"),
			Timeout:            2 * time.Second,
			AAKey:              "key",
		})
		if err == nil {
			t.Fatal("Collect() error = nil, want error (missing providers.toml)")
		}
		if srv.apiCount != 0 || srv.modelsCount != 0 {
			t.Errorf("apiCount=%d modelsCount=%d, want 0 (fail before any HTTP)", srv.apiCount, srv.modelsCount)
		}
	})

	t.Run("case 11-12: backup + atomic write + counters", func(t *testing.T) {
		installInsecureTransport(t)
		apiJSON := `[{"provider":"anthropic","id":"opus5","name":"Claude Opus 5"},{"provider":"openai","id":"gpt6","name":"GPT-6"}]`
		srv := newModelsDevTestServer(t, apiJSON, "[]")
		defer srv.Close()
		withModelsDevSeams(t, srv)
		aaSrv := newAATestServer(t, `{"pagination":{"page":1,"has_more":false},"data":[]}`)
		defer aaSrv.Close()
		withAASeam(t, aaSrv)

		dir := t.TempDir()
		benchPath := filepath.Join(dir, "b.toml")
		if err := os.WriteFile(benchPath, []byte("[benchmark_selection]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		outPath := filepath.Join(dir, "raw.csv")
		if err := os.WriteFile(outPath, []byte("model,reasoning\nOld,high\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		res, err := (defaultRunner{}).Collect(context.Background(), CollectOptions{
			ProviderConfigPath: "",
			BenchmarksPath:     benchPath,
			OutPath:            outPath,
			Timeout:            2 * time.Second,
			CacheTTL:           time.Hour,
			CatalogueCachePath: filepath.Join(dir, "cache.json"),
			AAKey:              "key",
		})
		if err != nil {
			t.Fatalf("Collect() error = %v", err)
		}
		if res.Providers != 2 {
			t.Errorf("Providers = %d, want 2", res.Providers)
		}
		if res.Models != 2 {
			t.Errorf("Models = %d, want 2", res.Models)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir() error = %v", err)
		}
		hasBak := false
		for _, e := range entries {
			if filepathHasBakSuffix(e.Name()) {
				hasBak = true
			}
		}
		if !hasBak {
			t.Error("no .bak file found after Collect on an existing raw.csv")
		}
		got, _, err := csvstore.Read(outPath)
		if err != nil {
			t.Fatalf("Read(outPath) error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("rows on disk = %d, want 2", len(got))
		}
	})
}

func filepathHasBakSuffix(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == ".bak"
}
