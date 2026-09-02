package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func TestParseCatalogRepoSpec(t *testing.T) {
	cases := []struct {
		in, owner, repo, ref string
	}{
		{"", "WD-Mitchell", "which-model", "main"},
		{"WD-Mitchell/which-model", "WD-Mitchell", "which-model", "main"},
		{"WD-Mitchell/which-model@dev", "WD-Mitchell", "which-model", "dev"},
		{"https://github.com/WD-Mitchell/which-model", "WD-Mitchell", "which-model", "main"},
		{"https://github.com/WD-Mitchell/which-model/tree/v2", "WD-Mitchell", "which-model", "v2"},
	}
	for _, tc := range cases {
		owner, repo, ref, err := config.ParseCatalogRepoSpec(tc.in)
		if err != nil {
			t.Fatalf("ParseCatalogRepoSpec(%q) error = %v", tc.in, err)
		}
		if owner != tc.owner || repo != tc.repo || ref != tc.ref {
			t.Fatalf("ParseCatalogRepoSpec(%q) = %s/%s@%s, want %s/%s@%s", tc.in, owner, repo, ref, tc.owner, tc.repo, tc.ref)
		}
	}
	if _, _, _, err := config.ParseCatalogRepoSpec("not a repo"); err == nil {
		t.Fatal("ParseCatalogRepoSpec(invalid) error = nil")
	}
}

func TestPullCatalogFromRepoWritesCache(t *testing.T) {
	svc, _ := newTestServices(t)
	scores := []byte("model,reasoning,intelligence_index_score\nClaude Opus 5,max,90\n")
	raw := []byte("model,reasoning,intelligence_index\nClaude Opus 5,max,70\n")
	bench := []byte("[benchmark_selection]\ngroups = [\"reasoning\"]\n")
	stubCatalogRepoFetch(t, map[string][]byte{
		catalogRepoScoresRel:     scores,
		catalogRepoRawRel:        raw,
		catalogRepoBenchmarksRel: bench,
	})
	if err := svc.pullCatalogFromRepo(context.Background(), "example/catalog"); err != nil {
		t.Fatalf("pullCatalogFromRepo: %v", err)
	}
	dir := filepath.Join(svc.paths.CacheDir, "catalog")
	gotScores, err := os.ReadFile(filepath.Join(dir, "available_model_scores.csv"))
	if err != nil || string(gotScores) != string(scores) {
		t.Fatalf("scores = %q, err %v", gotScores, err)
	}
	gotRaw, err := os.ReadFile(filepath.Join(dir, "available_model_raw_values.csv"))
	if err != nil || string(gotRaw) != string(raw) {
		t.Fatalf("raw = %q, err %v", gotRaw, err)
	}
	gotBench, err := os.ReadFile(filepath.Join(dir, "benchmarks.toml"))
	if err != nil || string(gotBench) != string(bench) {
		t.Fatalf("benchmarks = %q, err %v", gotBench, err)
	}
}

func TestSettingsPersistsAAKeySidecar(t *testing.T) {
	svc, _ := newTestServices(t)
	got, err := svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.AAAPIKeySet || got.AAAPIKey != "" {
		t.Fatalf("Get defaults AA = %+v", got)
	}
	got.AAAPIKey = "aa-secret-key"
	got.UseLocalAA = true
	if err := svc.Settings().Set(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.UseLocalAA || !got.AAAPIKeySet || got.AAAPIKey != "" {
		t.Fatalf("Get after set key = %+v", got)
	}
	if readAAKeyFile(svc.paths.ConfigDir) != "aa-secret-key" {
		t.Fatalf("sidecar = %q", readAAKeyFile(svc.paths.ConfigDir))
	}
	got.AAAPIKey = aaKeyClearSentinel
	if err := svc.Settings().Set(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.AAAPIKeySet {
		t.Fatal("key still set after clear")
	}
}

func stubCatalogRepoFetch(t *testing.T, files map[string][]byte) {
	t.Helper()
	prev := fetchCatalogRepoFile
	fetchCatalogRepoFile = func(_ context.Context, rawURL string) ([]byte, error) {
		for rel, body := range files {
			if strings.HasSuffix(rawURL, "/"+rel) {
				return append([]byte(nil), body...), nil
			}
		}
		return nil, fmt.Errorf("unexpected catalog url %s", rawURL)
	}
	t.Cleanup(func() { fetchCatalogRepoFile = prev })
}

func stubCatalogRepoFromCache(t *testing.T, svc *Services) {
	t.Helper()
	dir := filepath.Join(svc.paths.CacheDir, "catalog")
	scores, err := os.ReadFile(filepath.Join(dir, "available_model_scores.csv"))
	if err != nil {
		t.Fatal(err)
	}
	bench, err := os.ReadFile(filepath.Join(dir, "benchmarks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("model,reasoning\nClaude Opus 5,max\n")
	if b, err := os.ReadFile(filepath.Join(dir, "available_model_raw_values.csv")); err == nil {
		raw = b
	}
	stubCatalogRepoFetch(t, map[string][]byte{
		catalogRepoScoresRel:     scores,
		catalogRepoRawRel:        raw,
		catalogRepoBenchmarksRel: bench,
	})
}
