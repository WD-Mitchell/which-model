package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

// EmitFunc delivers an event to the host. Must be non-blocking (host's duty).
// service.New replaces nil with a no-op (B02 SPEC §2.1).
type EmitFunc func(event string, payload any)

// Services is the single instance the host owns (B02 SPEC §2). Zero value is
// unusable. One sync.RWMutex guards the config document, catalog caches, and
// routes table; read methods take RLock for the whole read (B00 SPEC §2.2).
type Services struct {
	harnessHome       string // local discovery home; empty uses the OS home; tests isolate it
	mu                sync.RWMutex
	dataRefreshMu     sync.Mutex
	paths             config.Paths
	cfg               *config.Config
	emit              EmitFunc
	scores            []catalog.ScoreRow
	rawValues         map[string]map[modelKey]decimal.Decimal // B05 §2.2: benchmark -> (model,reasoning) -> raw cell
	benchConfig       *score.BenchmarkConfig
	routes            routing.Table
	warnings          []string
	usageCacheDir     string
	usageFetchMu      sync.Mutex
	refresherOnce     sync.Once
	dataRefresherOnce sync.Once
	// recordPick records a profile pick after a successful harness launch
	// (B07 SPEC §2.10). Wired to B04's RecordPick by New; Launch logs (never
	// returns) a failure. Tests may override it.
	recordPick func(ctx context.Context, profileSlug, routeKey string) error
	// version is the read-only build identity reported through
	// GUISettings.Version ("" = unknown). Set by the host via WithVersion.
	version string
	// catalogRefresh, when set, rebuilds the scores CSV (models.dev +
	// Artificial Analysis) before RefreshRoutes joins it. The desktop host
	// wires this to `which-model catalog refresh`; tests leave it nil.
	catalogRefresh CatalogRefreshFunc
}

// CatalogRefreshFunc rebuilds the on-disk catalogue (raw CSV + derived
// scores). It must not hold Services.mu; the caller reloads caches after it
// returns. Failure is returned to Refresh models; sign-in ignores it.
type CatalogRefreshFunc func(ctx context.Context) error

// SetCatalogRefresh installs the host's catalogue rebuild hook. Call once
// after New/NewEmpty; not concurrent with RefreshRoutes.
func (s *Services) SetCatalogRefresh(fn CatalogRefreshFunc) {
	s.catalogRefresh = fn
}

// catalogConfig uses the complete shared schema, including publishing options.
type catalogConfig = catalog.Config

// Sentinel errors; features wrap them (fmt.Errorf("%w: ...", errValidation))
// so toErrorDTO recovers the code via errors.Is (B02 CONTRACTS §3).
var (
	errValidation       = errors.New("validation failed")
	errBuiltinReadonly  = errors.New("builtin is read-only")
	errNotFound         = errors.New("not found")
	errConflict         = errors.New("already exists")
	errUsageUnavailable = errors.New("usage unavailable")
	errLaunchFailed     = errors.New("launch failed")
	// errScoresMissing is retained as a sentinel for callers that need to
	// distinguish a missing scores artifact from other I/O failures. The
	// typed scoresMissingError below also unwraps fs.ErrNotExist.
	errScoresMissing = errors.New("scores CSV missing")
)

// scoresMissingError is the typed fail-fast error for a missing scores CSV
// (B02 SPEC §2.3); it wraps fs.ErrNotExist and renders the CONTRACTS §7
// message. Maps to io_error.
type scoresMissingError struct{ Path string }

func (e *scoresMissingError) Error() string {
	return "scores CSV not found at " + e.Path + "; run: which-model catalog refresh"
}

func (e *scoresMissingError) Unwrap() error { return fs.ErrNotExist }

// Is lets errors.Is recognize the task-level errScoresMissing sentinel while
// preserving the CONTRACTS §3 Unwrap -> fs.ErrNotExist behavior.
func (e *scoresMissingError) Is(target error) bool {
	return target == errScoresMissing
}

// New eagerly loads scores CSV, benchmarks config, and routes table in that
// order, failing fast on the first error (B02 SPEC §2.2–2.5). A missing
// routes table is non-fatal (empty availability + warning, CONTRACTS §7). A
// nil emit is replaced with a no-op. New never re-reads config.toml.
func New(paths config.Paths, cfg *config.Config, emit EmitFunc) (*Services, error) {
	if emit == nil {
		emit = func(string, any) {}
	}
	s := &Services{
		paths: paths,
		cfg:   cfg,
		emit:  emit,
	}
	s.recordPick = s.RecordPick // B07 §2.10 seam wired to B04's RecordPick
	if err := s.reloadCatalog(); err != nil {
		return nil, err
	}
	return s, nil
}

// NewEmpty creates a Services with an empty catalog (no scores, no benchmarks,
// no routes). Used by the desktop app when the model catalog hasn't been
// refreshed yet — the UI shows an empty picker with a prompt to run
// `which-model catalog refresh`. All service methods work; ranking simply
// returns zero candidates.
func NewEmpty(paths config.Paths, cfg *config.Config, emit EmitFunc) *Services {
	if emit == nil {
		emit = func(string, any) {}
	}
	s := &Services{
		paths:    paths,
		cfg:      cfg,
		emit:     emit,
		warnings: []string{"catalog not loaded: run `which-model catalog refresh` to populate"},
	}
	s.recordPick = s.RecordPick
	return s
}

// TestOption and host-option seam: WithVersion sets the read-only build
// identity surfaced through GUISettings.Version. Empty means unknown.
func WithVersion(v string) func(*Services) {
	return func(s *Services) { s.version = v }
}

// SetVersion applies an option to a live Services value. The desktop host
// builds its Services first and learns its ldflags version alongside, so an
// apply-after-build keeps the constructor signatures unchanged.
func (s *Services) SetVersion(v string) { s.version = v }

// Warnings returns non-fatal construction warnings (currently only the
// missing-routes-table warning, CONTRACTS §7), copied, in occurrence order.
func (s *Services) Warnings() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.warnings...)
}

// reloadCatalog re-reads scores CSV + benchmarks config + routes table and
// swaps all three caches atomically under the write lock; on error the
// previous caches stay live (B02 SPEC §2.10). Emits nothing.
// ReloadCatalog re-reads the scores CSV, benchmark config and route table from
// disk and emits catalog:changed. Exported for hosts that rebuild the catalogue
// out of band (the desktop's Refresh data menu item) and need the running
// process to pick it up without a restart.
func (s *Services) ReloadCatalog() error {
	if err := s.reloadCatalog(); err != nil {
		return err
	}
	s.emit(EventCatalogChanged, map[string]any{})
	return nil
}

func (s *Services) reloadCatalog() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	scoresPath := filepath.Join(s.paths.CacheDir, "catalog", "available_model_scores.csv")

	// (a) Scores CSV — missing is the typed fail-fast error.
	data, err := os.ReadFile(scoresPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &scoresMissingError{Path: scoresPath}
		}
		return err
	}
	scores, err := score.ParseScoresCSV(data)
	if err != nil {
		return err
	}

	// (b) Benchmarks config — config key catalog.benchmark_config_path, else
	// <CacheDir>/catalog/benchmarks.toml. Missing or invalid fails fast.
	benchmarksPath := filepath.Join(s.paths.CacheDir, "catalog", "benchmarks.toml")
	if p := s.catalogBenchmarkPath(); p != "" {
		benchmarksPath = p
	}
	benchData, err := os.ReadFile(benchmarksPath)
	if err != nil {
		return err
	}
	bench, err := score.ParseBenchmarkConfig(benchData)
	if err != nil {
		return err
	}

	// (c) Routes table — primary + legacy fallback; missing is non-fatal.
	routes, warnings, err := s.loadRoutes()
	if err != nil {
		return err
	}

	// (d) B05: parse the scores CSV's raw benchmark cells (SPEC §2.2). The
	// scores already parsed, so a raw-parse failure only happens on a
	// malformed raw cell; fail the reload to keep reads consistent.
	rawValues, err := rawBenchmarkValues(data)
	if err != nil {
		return err
	}

	s.scores = scores
	s.rawValues = rawValues
	s.benchConfig = bench
	s.routes = routes
	s.warnings = warnings
	if err := applyCatalogOverlay(s.scores, s.cfg); err != nil {
		return err
	}
	return nil
}

// catalogBenchmarkPath reads [catalog].benchmark_config_path from the already
// loaded config; "" means the key is unset.
func (s *Services) catalogBenchmarkPath() string {
	if s.cfg == nil {
		return ""
	}
	var cc catalogConfig
	if err := s.cfg.UnmarshalKey("catalog", &cc); err != nil {
		return ""
	}
	return cc.BenchmarkConfigPath
}

// loadRoutes loads the routes table from the canonical <CacheDir>/routes.json
// (F18 SPEC §2.11 — the same location the CLI writes), falling back to the
// desktop's legacy <CacheDir>/catalog/routes.json when the canonical file is
// absent. A legacy table is migrated to the canonical path on load; a failed
// migration degrades to a warning. A missing table (both paths) is non-fatal:
// empty table + warning. A present-but-corrupt table is fatal (B02 SPEC §2.5).
func (s *Services) loadRoutes() (routing.Table, []string, error) {
	primary := filepath.Join(s.paths.CacheDir, "routes.json")
	table, err := routing.LoadTable(primary)
	if err == nil {
		return table, nil, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return routing.Table{}, nil, err
	}

	legacy := filepath.Join(s.paths.CacheDir, "catalog", "routes.json")
	table, err = routing.LoadTable(legacy)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return routing.Table{}, nil, err
		}
		warning := fmt.Sprintf("routes table not found at %s; availability is empty until: which-model routes refresh", primary)
		return routing.Table{}, []string{warning}, nil
	}

	warnings := []string(nil)
	if migrateErr := routing.SaveTable(primary, table); migrateErr != nil {
		warnings = append(warnings, fmt.Sprintf("routes table migration from %s to %s failed: %v", legacy, primary, migrateErr))
	}
	return table, warnings, nil
}

// saveRoutesLocked persists the in-memory route table to the canonical
// routes path. Callers hold s.mu for writing. Used when deleting a provider,
// whose routes must go with it — the provider universe is unioned FROM the
// route table, so leaving them would resurrect the row.
func (s *Services) saveRoutesLocked() error {
	path := filepath.Join(s.paths.CacheDir, "routes.json")
	return routing.SaveTable(path, s.routes)
}

// toErrorDTO maps any internal error to the boundary shape (B02 SPEC §2.7):
// an ErrorDTO (or *ErrorDTO) passes through; then errors.Is against the
// sentinels per CONTRACTS §5; then usage ctx errors -> usage_unavailable;
// everything else -> io_error with a sanitised message.
// ToErrorDTO exposes the canonical boundary mapping to native binding adapters.
func ToErrorDTO(err error) ErrorDTO { return toErrorDTO(err) }

func toErrorDTO(err error) ErrorDTO {
	if err == nil {
		return ErrorDTO{Code: "", Message: ""}
	}
	var dto ErrorDTO
	if errors.As(err, &dto) {
		return dto
	}
	var dtoPtr *ErrorDTO
	if errors.As(err, &dtoPtr) {
		return *dtoPtr
	}
	switch {
	case errors.Is(err, errValidation):
		return ErrorDTO{Code: "validation_failed", Message: err.Error()}
	case errors.Is(err, errBuiltinReadonly):
		return ErrorDTO{Code: "builtin_readonly", Message: err.Error()}
	case errors.Is(err, errNotFound):
		return ErrorDTO{Code: "not_found", Message: err.Error()}
	case errors.Is(err, errConflict):
		return ErrorDTO{Code: "conflict", Message: err.Error()}
	case errors.Is(err, errUsageUnavailable):
		return ErrorDTO{Code: "usage_unavailable", Message: err.Error()}
	case errors.Is(err, errLaunchFailed):
		return ErrorDTO{Code: "launch_failed", Message: err.Error()}
	default:
		return ErrorDTO{Code: "io_error", Message: sanitiseMessage(err.Error())}
	}
}

// Error makes ErrorDTO returnable directly from bound methods
// ("<code>: <message>").
func (e ErrorDTO) Error() string {
	return e.Code + ": " + e.Message
}

// sanitiseMessage scrubs common absolute home-directory prefixes from an
// error message without consulting process-global home state (B00 §2.8:
// service paths come only from injected config.Paths). The config path
// exception is intentionally not guessed here because this free function has
// no Paths context.
func sanitiseMessage(s string) string {
	for _, prefix := range []string{"/Users/", "/home/"} {
		for {
			i := strings.Index(s, prefix)
			if i < 0 {
				break
			}
			userStart := i + len(prefix)
			rest := s[userStart:]
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				s = s[:i] + "~" + rest[slash:]
			} else {
				s = s[:i] + "~"
			}
		}
	}
	return s
}

// dtoWeights converts engine decimals to DTO ints: each value Round(0) half-up
// to int; keys rounding to <=0 are dropped (B02 SPEC §2.9).
func dtoWeights(m map[string]decimal.Decimal) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		iv := v.Round(0).IntPart()
		if iv <= 0 {
			continue
		}
		out[k] = int(iv)
	}
	return out
}

// engineWeights converts DTO ints to engine decimals: keys with v<=0 removed;
// any v>5 -> errValidation, checking keys in sorted order (deterministic).
func engineWeights(m map[string]int) (map[string]decimal.Decimal, error) {
	if len(m) == 0 {
		return map[string]decimal.Decimal{}, nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]decimal.Decimal, len(m))
	for _, k := range keys {
		v := m[k]
		if v <= 0 {
			continue
		}
		if v > 5 {
			return nil, fmt.Errorf("%w: weight %q is %d, must be 0..5", errValidation, k, v)
		}
		out[k] = decimal.NewFromInt(int64(v))
	}
	return out, nil
}

// engineProfile builds a catalog.Profile from a ProfileDetail:
// Tier1Share = CoreShare/100 (decimal, no rounding), Tier2Share = 1-Tier1Share,
// Name = Slug, weights via engineWeights (tier1 error checked first).
func engineProfile(d ProfileDetail) (catalog.Profile, error) {
	tier1 := decimal.NewFromInt(int64(d.CoreShare)).Div(decimal.NewFromInt(100))
	w1, err := engineWeights(d.Tier1Weights)
	if err != nil {
		return catalog.Profile{}, err
	}
	w2, err := engineWeights(d.Tier2Weights)
	if err != nil {
		return catalog.Profile{}, err
	}
	return catalog.Profile{
		Name:         d.Slug,
		Tier1Share:   tier1,
		Tier2Share:   decimal.NewFromInt(1).Sub(tier1),
		Tier1Weights: w1,
		Tier2Weights: w2,
	}, nil
}

// round2 is the ONLY decimal->float64 crossing: d.Round(2).InexactFloat64().
func round2(d decimal.Decimal) float64 {
	return d.Round(2).InexactFloat64()
}
