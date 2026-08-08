package modelsdev

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

func TestFetchModelsDevBenchmarksSelection(t *testing.T) {
	resetRecorded()
	payload := `[
		{"id":"anthropic/claude-opus-5","name":"Claude Opus 5","variant":"medium effort",
		 "SWE-Bench Verified":{"score":0.63,"version":"v1"},
		 "SWE-Bench Pro":{"score":0.5},
		 "DeepSWE":0.4},
		{"id":"openai/gpt-5.6","name":"GPT-5.6 Sol",
		 "SWE-Bench Verified":"0.62",
		 "SWE-Bench Pro":{"score":0.51},
		 "DeepSWE":{"score":0.41}}
	]`
	srv := newTestServer(t, payload, http.StatusOK)
	got, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified", "DeepSWE"})
	if err != nil {
		t.Fatalf("FetchModelsDevBenchmarksFrom: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	seen := map[string]bool{}
	for _, rec := range got {
		if rec.CanonicalID != "anthropic/claude-opus-5" && rec.CanonicalID != "openai/gpt-5.6" {
			t.Errorf("unexpected CanonicalID %q", rec.CanonicalID)
		}
		seen[rec.CanonicalID] = true
		if rec.Name == "" {
			t.Error("Name empty")
		}
		if len(rec.Benchmarks) != 2 {
			t.Errorf("%s: got %d benchmarks, want 2 (unselected SWE-Bench Pro dropped)", rec.CanonicalID, len(rec.Benchmarks))
		}
		for _, ev := range rec.Benchmarks {
			if ev.Name != "SWE-Bench Verified" && ev.Name != "DeepSWE" {
				t.Errorf("%s: unexpected benchmark %q", rec.CanonicalID, ev.Name)
			}
			if ev.Score.Sign() <= 0 {
				t.Errorf("%s: non-positive score %v", rec.CanonicalID, ev.Score)
			}
		}
	}
	if !seen["anthropic/claude-opus-5"] || !seen["openai/gpt-5.6"] {
		t.Errorf("missing records: %v", seen)
	}

	// The claude record uses an object {"score":...}, the gpt record mixes a
	// bare number and a numeric string; both must decode.
	var claude, gpt *BenchmarkRecord
	for i := range got {
		switch got[i].CanonicalID {
		case "anthropic/claude-opus-5":
			claude = &got[i]
		case "openai/gpt-5.6":
			gpt = &got[i]
		}
	}
	if claude == nil || gpt == nil {
		t.Fatal("missing expected records")
	}
	for _, ev := range claude.Benchmarks {
		if ev.Name == "SWE-Bench Verified" && !ev.Score.Equal(decimalFromString(t, "0.63")) {
			t.Errorf("claude SWE-Bench Verified = %v, want 0.63", ev.Score)
		}
	}
	for _, ev := range gpt.Benchmarks {
		if ev.Name == "SWE-Bench Verified" && !ev.Score.Equal(decimalFromString(t, "0.62")) {
			t.Errorf("gpt SWE-Bench Verified = %v, want 0.62 (numeric string)", ev.Score)
		}
	}
}

func decimalFromString(t *testing.T, s string) sdecimal.Decimal {
	t.Helper()
	d, err := sdecimal.NewFromString(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func TestFetchModelsDevBenchmarksEffort(t *testing.T) {
	payload := `[
		{"id":"m-low","name":"M","variant":"low effort","SWE-Bench Verified":{"score":0.5}},
		{"id":"m-none","name":"M","variant":"reasoning effort none","SWE-Bench Verified":{"score":0.6}},
		{"id":"m-tools","name":"M","variant":"with tools","SWE-Bench Verified":{"score":0.7}}
	]`
	srv := newTestServer(t, payload, http.StatusOK)
	got, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified"})
	if err != nil {
		t.Fatalf("FetchModelsDevBenchmarksFrom: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	want := map[string]string{"m-low": "low", "m-none": "default", "m-tools": ""}
	for _, rec := range got {
		if len(rec.Benchmarks) != 1 {
			t.Errorf("%s: got %d benchmarks, want 1", rec.CanonicalID, len(rec.Benchmarks))
			continue
		}
		if gotEffort := rec.Benchmarks[0].Effort; gotEffort != want[rec.CanonicalID] {
			t.Errorf("%s: Effort = %q, want %q", rec.CanonicalID, gotEffort, want[rec.CanonicalID])
		}
	}
}

func TestFetchModelsDevBenchmarksMaxWins(t *testing.T) {
	payload := `[
		{"id":"m1","name":"M1","variant":"medium effort","SWE-Bench Verified":{"score":0.63}},
		{"id":"m1","name":"M1","variant":"medium effort","SWE-Bench Verified":{"score":0.88}},
		{"id":"m2","name":"M2","SWE-Bench Verified":{"score":0.5}},
		{"id":"m2","name":"M2","variant":"low effort","SWE-Bench Verified":{"score":0.6}}
	]`
	srv := newTestServer(t, payload, http.StatusOK)
	got, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified"})
	if err != nil {
		t.Fatalf("FetchModelsDevBenchmarksFrom: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (duplicate model ids merged)", len(got))
	}

	// m1: same (benchmark, effort) twice -> single evidence, max score 0.88.
	var m1 *BenchmarkRecord
	for i := range got {
		if got[i].CanonicalID == "m1" {
			m1 = &got[i]
		}
	}
	if m1 == nil {
		t.Fatal("m1 missing")
	}
	if len(m1.Benchmarks) != 1 {
		t.Fatalf("m1 has %d evidences, want 1 (max wins, never priority-list)", len(m1.Benchmarks))
	}
	if !m1.Benchmarks[0].Score.Equal(decimalFromString(t, "0.88")) {
		t.Errorf("m1 score = %v, want 0.88 (max)", m1.Benchmarks[0].Score)
	}
	if m1.Benchmarks[0].Effort != "medium" {
		t.Errorf("m1 Effort = %q, want medium", m1.Benchmarks[0].Effort)
	}

	// m2: an evidence at (name, "") from the variant-less record is kept as a
	// separate entry from the effort-scoped one.
	var m2 *BenchmarkRecord
	for i := range got {
		if got[i].CanonicalID == "m2" {
			m2 = &got[i]
		}
	}
	if m2 == nil {
		t.Fatal("m2 missing")
	}
	if len(m2.Benchmarks) != 2 {
		t.Fatalf("m2 has %d evidences, want 2 (effort-scoped vs unscoped)", len(m2.Benchmarks))
	}
	scores := map[string]string{}
	for _, ev := range m2.Benchmarks {
		scores[ev.Effort] = ev.Score.String()
	}
	if scores[""] != "0.5" || scores["low"] != "0.6" {
		t.Errorf("m2 scores by effort = %v, want empty->0.5 low->0.6", scores)
	}
}

func TestFetchModelsDevBenchmarksEmptySelection(t *testing.T) {
	payload := `[
		{"id":"m1","name":"M1","variant":"medium effort","SWE-Bench Verified":{"score":0.63}},
		{"id":"m2","name":"M2","SWE-Bench Verified":{"score":0.5}}
	]`
	srv := newTestServer(t, payload, http.StatusOK)
	got, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, nil)
	if err != nil {
		t.Fatalf("FetchModelsDevBenchmarksFrom: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	for _, rec := range got {
		if len(rec.Benchmarks) != 0 {
			t.Errorf("%s: got %d benchmarks, want 0 (empty selection)", rec.CanonicalID, len(rec.Benchmarks))
		}
	}
}

func TestBenchmarksURLConstant(t *testing.T) {
	if BenchmarksURL != "https://models.dev/models.json" {
		t.Errorf("BenchmarksURL = %q, want https://models.dev/models.json", BenchmarksURL)
	}
	if BenchmarksURL == ProvidersURL {
		t.Error("BenchmarksURL must be distinct from ProvidersURL")
	}
}

// TestFetchModelsDevFileIsolation pins the T3/T4 file isolation: benchmark.go
// must not reference provider-file symbols and vice versa (source-text check,
// mirror of T2's provider-side test).
func TestFetchModelsDevFileIsolation(t *testing.T) {
	benchmarkSrc, err := os.ReadFile("benchmark.go")
	if err != nil {
		t.Fatalf("read benchmark.go: %v", err)
	}
	providerSrc, err := os.ReadFile("provider.go")
	if err != nil {
		t.Fatalf("read provider.go: %v", err)
	}
	for _, tok := range []string{"FetchModelsDevProviders", "ProvidersURL", "ProviderModel"} {
		if strings.Contains(string(benchmarkSrc), tok) {
			t.Errorf("benchmark.go references provider-file symbol %q", tok)
		}
	}
	for _, tok := range []string{"FetchModelsDevBenchmarks", "BenchmarksURL", "BenchmarkRecord"} {
		if strings.Contains(string(providerSrc), tok) {
			t.Errorf("provider.go references benchmark-file symbol %q", tok)
		}
	}
}

func TestFetchModelsDevBenchmarksErrors(t *testing.T) {
	t.Run("http 500 -> provider_status", func(t *testing.T) {
		resetRecorded()
		srv := newTestServer(t, `[]`, http.StatusInternalServerError)
		_, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified"})
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "provider_status" {
			t.Errorf("Code = %q, want provider_status", fe.Code)
		}
	})

	t.Run("not json -> response_json", func(t *testing.T) {
		srv := newTestServer(t, `nope`, http.StatusOK)
		_, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, nil)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "response_json" {
			t.Errorf("Code = %q, want response_json", fe.Code)
		}
	})

	t.Run("non-numeric score -> unsupported_response", func(t *testing.T) {
		srv := newTestServer(t, `[{"id":"m","name":"M","SWE-Bench Verified":{"score":"abc"}}]`, http.StatusOK)
		_, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified"})
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})

	t.Run("negative score -> unsupported_response", func(t *testing.T) {
		srv := newTestServer(t, `[{"id":"m","name":"M","SWE-Bench Verified":{"score":-0.1}}]`, http.StatusOK)
		_, err := FetchModelsDevBenchmarksFrom(httpkit.NewClient(), srv.URL, []string{"SWE-Bench Verified"})
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "unsupported_response" {
			t.Errorf("Code = %q, want unsupported_response", fe.Code)
		}
	})
}
