package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// TestOption mutates the test fixture before newTestServices materialises it
// (B02 CONTRACTS §9).
type TestOption func(*testFixture)

type testFixture struct {
	paths      config.Paths
	configTOML string
	scoresCSV  string
	omitScores bool
	routes     *routing.Table
}

// WithConfigTOML replaces the config.toml content (default: empty).
func WithConfigTOML(s string) TestOption {
	return func(f *testFixture) { f.configTOML = s }
}

// WithScoresCSV replaces the fixture scores CSV; "" omits the file.
func WithScoresCSV(csv string) TestOption {
	return func(f *testFixture) {
		if csv == "" {
			f.omitScores = true
			return
		}
		f.scoresCSV = csv
	}
}

// WithRoutes replaces the synthetic routes table.
func WithRoutes(rt routing.Table) TestOption {
	return func(f *testFixture) { f.routes = &rt }
}

// recordedEvent is one captured emit (B02 CONTRACTS §9).
type recordedEvent struct {
	Event   string
	Payload any
}

// emitRecorder captures emitted events; Events() is safe to call concurrently.
type emitRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

// emit appends one event (the EmitFunc handed to New).
func (r *emitRecorder) emit(event string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{Event: event, Payload: payload})
}

// Events returns a copy of the recorded events.
func (r *emitRecorder) Events() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent(nil), r.events...)
}

// newTestServices builds a t.TempDir() config/cache/state tree, writes the
// default fixtures, applies opts, loads config, and calls New (B02 CONTRACTS
// §9). Fatal on any setup or New error.
func newTestServices(t *testing.T, opts ...TestOption) (*Services, *emitRecorder) {
	t.Helper()
	paths, cfg := materializeFixture(t, opts...)
	rec := &emitRecorder{}
	svc, err := New(paths, cfg, rec.emit)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc, rec
}

// materializeFixture writes the fixture tree and loads config. Used directly
// by tests that must exercise New's own error paths (e.g. missing scores CSV).
func materializeFixture(t *testing.T, opts ...TestOption) (config.Paths, *config.Config) {
	t.Helper()
	root := t.TempDir()
	fix := &testFixture{
		paths: config.Paths{
			UserConfigFile: filepath.Join(root, "config", "config.toml"),
			ConfigDir:      filepath.Join(root, "config"),
			CacheDir:       filepath.Join(root, "cache"),
			StateDir:       filepath.Join(root, "state"),
		},
	}
	for _, opt := range opts {
		opt(fix)
	}
	if err := writeFixtureTree(fix); err != nil {
		t.Fatalf("write fixture tree: %v", err)
	}
	cfg, err := config.Load(config.LoadOptions{Path: fix.paths.UserConfigFile, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return fix.paths, cfg
}

// writeFixtureTree creates the full default fixture tree under fix.paths and
// materialises any option overrides.
func writeFixtureTree(fix *testFixture) error {
	dirs := []string{
		filepath.Join(fix.paths.CacheDir, "catalog"),
		fix.paths.ConfigDir,
		fix.paths.StateDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}

	// config.toml — empty by default.
	if err := os.WriteFile(fix.paths.UserConfigFile, []byte(fix.configTOML), 0o600); err != nil {
		return err
	}

	// scores CSV — default byte-copy of scores_golden.csv, or override, or omitted.
	if !fix.omitScores {
		scores := fix.scoresCSV
		if scores == "" {
			data, err := os.ReadFile(filepath.Join("..", "catalog", "score", "testdata", "scores_golden.csv"))
			if err != nil {
				return err
			}
			scores = string(data)
		}
		if err := os.WriteFile(filepath.Join(fix.paths.CacheDir, "catalog", "available_model_scores.csv"), []byte(scores), 0o600); err != nil {
			return err
		}
	}

	// benchmarks.toml — copy of the golden benchmark config.
	bench, err := os.ReadFile(filepath.Join("..", "catalog", "score", "testdata", "benchmarks_golden.toml"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(fix.paths.CacheDir, "catalog", "benchmarks.toml"), bench, 0o600); err != nil {
		return err
	}

	// providers.toml — mirror of the fixture tree requirement (not consumed by
	// B02's New, but written so later features have it).
	if err := os.WriteFile(filepath.Join(fix.paths.ConfigDir, "providers.toml"), []byte("# providers fixture\n"), 0o600); err != nil {
		return err
	}

	// routes table — synthetic default or the option-supplied table.
	table := defaultRoutes()
	if fix.routes != nil {
		table = *fix.routes
	}
	return routing.SaveTable(filepath.Join(fix.paths.CacheDir, "routes.json"), table)
}

// defaultRoutes builds the CONTRACTS §9 synthetic routes table: providers
// claude,codex × ≥2 models × ≥2 reasoning levels each.
func defaultRoutes() routing.Table {
	return routing.Table{
		SchemaVersion: routing.TableSchemaVersion,
		Routes: []routing.Route{
			{Provider: "claude", ModelID: "claude-opus-5", Model: "Claude Opus 5", Reasoning: "high", Provenance: routing.ProvenanceProviderLive},
			{Provider: "claude", ModelID: "claude-opus-5", Model: "Claude Opus 5", Reasoning: "max", Provenance: routing.ProvenanceProviderLive},
			{Provider: "claude", ModelID: "claude-sonnet-4", Model: "Claude Sonnet 4", Reasoning: "medium", Provenance: routing.ProvenanceProviderLive},
			{Provider: "codex", ModelID: "gpt-5.6", Model: "GPT-5.6 Sol", Reasoning: "high", Provenance: routing.ProvenanceProviderLive},
			{Provider: "codex", ModelID: "gpt-5.6", Model: "GPT-5.6 Sol", Reasoning: "medium", Provenance: routing.ProvenanceProviderLive},
			{Provider: "codex", ModelID: "gpt-4.2", Model: "GPT-4.2", Reasoning: "low", Provenance: routing.ProvenanceProviderLive},
		},
	}
}
