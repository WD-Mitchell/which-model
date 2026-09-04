package service

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/routing"
)

func TestNew_LoadsFixtures(t *testing.T) {
	svc, rec := newTestServices(t)

	if len(svc.scores) != 3 {
		t.Errorf("scores = %d rows, want 3 (scores_golden)", len(svc.scores))
	}
	if svc.scores[0].Model != "Claude Opus 5" || svc.scores[0].Reasoning != "max" {
		t.Errorf("scores[0] = %q/%q, want Claude Opus 5/max", svc.scores[0].Model, svc.scores[0].Reasoning)
	}
	if svc.benchConfig == nil {
		t.Fatal("benchConfig = nil, want parsed benchmark config")
	}
	if len(svc.benchConfig.CanonicalBenchmarks) == 0 {
		t.Error("benchConfig.CanonicalBenchmarks empty, want populated")
	}
	if svc.routes.SchemaVersion != routing.TableSchemaVersion {
		t.Errorf("routes.SchemaVersion = %q, want %q", svc.routes.SchemaVersion, routing.TableSchemaVersion)
	}
	if len(svc.routes.Routes) == 0 {
		t.Error("routes.Routes empty, want synthetic fixture")
	}
	if w := svc.Warnings(); len(w) != 0 {
		t.Errorf("Warnings() = %v, want empty", w)
	}
	if ev := rec.Events(); len(ev) != 0 {
		t.Errorf("events = %v, want zero emitted by New", ev)
	}
}

func TestNew_MissingScoresCSV(t *testing.T) {
	paths, cfg := materializeFixture(t, WithScoresCSV(""))
	rec := &emitRecorder{}
	_, err := New(paths, cfg, rec.emit)
	if err == nil {
		t.Fatal("New() error = nil, want errScoresMissing")
	}
	dto := toErrorDTO(err)
	if dto.Code != "io_error" {
		t.Errorf("toErrorDTO code = %q, want io_error", dto.Code)
	}
	want := "scores CSV not found at " + filepath.Join(paths.CacheDir, "catalog", "available_model_scores.csv") + "; run: which-model catalog refresh"
	if !strings.Contains(dto.Message, want) {
		t.Errorf("message = %q, want to contain %q", dto.Message, want)
	}
	var missing *scoresMissingError
	if !errors.As(err, &missing) {
		t.Errorf("err type = %T, want *scoresMissingError", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Error("errors.Is(err, os.ErrNotExist) = false, want true (Unwrap -> fs.ErrNotExist)")
	}
	if !errors.Is(err, errScoresMissing) {
		t.Error("errors.Is(err, errScoresMissing) = false, want true")
	}
	if ev := rec.Events(); len(ev) != 0 {
		t.Errorf("events = %v, want zero on failed New", ev)
	}
}

func TestNew_MissingRoutesTable(t *testing.T) {
	paths, cfg := materializeFixture(t)
	// Remove the canonical routes table -> non-fatal.
	os.Remove(filepath.Join(paths.CacheDir, "routes.json"))

	rec := &emitRecorder{}
	svc, err := New(paths, cfg, rec.emit)
	if err != nil {
		t.Fatalf("New() error = %v, want success with empty table", err)
	}
	if len(svc.routes.Routes) != 0 {
		t.Errorf("routes.Routes = %d, want empty", len(svc.routes.Routes))
	}
	want := "routes table not found at " + filepath.Join(paths.CacheDir, "routes.json") + "; availability is empty until: which-model routes refresh"
	w := svc.Warnings()
	if len(w) != 1 {
		t.Fatalf("Warnings() = %v, want exactly one entry", w)
	}
	if w[0] != want {
		t.Errorf("warning = %q, want %q", w[0], want)
	}
	if len(svc.scores) != 3 {
		t.Errorf("scores = %d rows, want 3 (load continues past missing routes)", len(svc.scores))
	}
}

// A table at the desktop's old <cache>/catalog/routes.json location is
// migrated to the canonical <cache>/routes.json path on first load.
func TestNew_RoutesLegacyMigration(t *testing.T) {
	paths, cfg := materializeFixture(t)
	if err := os.Remove(filepath.Join(paths.CacheDir, "routes.json")); err != nil {
		t.Fatal(err)
	}
	legacy := defaultRoutes()
	legacy.Routes = legacy.Routes[:1]
	if err := routing.SaveTable(filepath.Join(paths.CacheDir, "catalog", "routes.json"), legacy); err != nil {
		t.Fatal(err)
	}

	rec := &emitRecorder{}
	svc, err := New(paths, cfg, rec.emit)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(svc.routes.Routes) != 1 || svc.routes.Routes[0].ModelID != legacy.Routes[0].ModelID {
		t.Errorf("routes = %d entries, want the legacy table's single route", len(svc.routes.Routes))
	}
	if w := svc.Warnings(); len(w) != 0 {
		t.Errorf("Warnings() = %v, want empty after migration", w)
	}
	migrated, err := routing.LoadTable(filepath.Join(paths.CacheDir, "routes.json"))
	if err != nil {
		t.Fatalf("canonical table after migration: %v", err)
	}
	if len(migrated.Routes) != 1 || migrated.Routes[0].ModelID != legacy.Routes[0].ModelID {
		t.Errorf("migrated table = %d entries, want the legacy table's single route", len(migrated.Routes))
	}
}

// When both locations exist, the canonical <cache>/routes.json wins and the
// legacy copy is left alone.
func TestNew_RoutesCanonicalPreferred(t *testing.T) {
	paths, cfg := materializeFixture(t)
	legacy := defaultRoutes()
	legacy.Routes = legacy.Routes[:1]
	if err := routing.SaveTable(filepath.Join(paths.CacheDir, "catalog", "routes.json"), legacy); err != nil {
		t.Fatal(err)
	}

	rec := &emitRecorder{}
	svc, err := New(paths, cfg, rec.emit)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(svc.routes.Routes) != len(defaultRoutes().Routes) {
		t.Errorf("routes = %d entries, want the canonical table's %d", len(svc.routes.Routes), len(defaultRoutes().Routes))
	}
	if w := svc.Warnings(); len(w) != 0 {
		t.Errorf("Warnings() = %v, want empty", w)
	}
}

// saveRoutesLocked persists to the canonical path only.
func TestSaveRoutesWritesCanonical(t *testing.T) {
	svc, _ := newTestServices(t)
	svc.mu.Lock()
	err := svc.saveRoutesLocked()
	svc.mu.Unlock()
	if err != nil {
		t.Fatalf("saveRoutesLocked() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.CacheDir, "routes.json")); err != nil {
		t.Errorf("canonical routes file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.paths.CacheDir, "catalog", "routes.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("legacy catalog/routes.json stat = %v, want not exist", err)
	}
}

func TestNew_CorruptScoresCSV(t *testing.T) {
	paths, cfg := materializeFixture(t, WithScoresCSV("this,is,not,a,scores,header\n"))
	rec := &emitRecorder{}
	_, err := New(paths, cfg, rec.emit)
	if err == nil {
		t.Fatal("New() error = nil, want corrupt-scores error")
	}
	if dto := toErrorDTO(err); dto.Code != "io_error" {
		t.Errorf("toErrorDTO code = %q, want io_error", dto.Code)
	}
}

func TestNew_CorruptBenchmarks(t *testing.T) {
	paths, cfg := materializeFixture(t)
	if err := os.WriteFile(filepath.Join(paths.CacheDir, "catalog", "benchmarks.toml"), []byte("]]]not valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := &emitRecorder{}
	_, err := New(paths, cfg, rec.emit)
	if err == nil {
		t.Fatal("New() error = nil, want corrupt-benchmark error")
	}
	if dto := toErrorDTO(err); dto.Code != "io_error" {
		t.Errorf("toErrorDTO code = %q, want io_error", dto.Code)
	}
}

func TestNew_CorruptRoutesTable(t *testing.T) {
	paths, cfg := materializeFixture(t)
	if err := os.WriteFile(filepath.Join(paths.CacheDir, "routes.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := &emitRecorder{}
	_, err := New(paths, cfg, rec.emit)
	if err == nil {
		t.Fatal("New() error = nil, want corrupt-routes error")
	}
	dto := toErrorDTO(err)
	if dto.Code != "io_error" {
		t.Errorf("toErrorDTO code = %q, want io_error", dto.Code)
	}
	if !strings.Contains(err.Error(), filepath.Join(paths.CacheDir, "routes.json")) {
		t.Errorf("error message = %q, want to name the corrupt path", err.Error())
	}
}

func TestRouteKey_RoundTrip(t *testing.T) {
	for _, rt := range defaultRoutes().Routes {
		key := FormatRouteKey(rt.Provider, rt.ModelID, rt.Reasoning)
		provider, modelID, reasoning, err := ParseRouteKey(key)
		if err != nil {
			t.Errorf("ParseRouteKey(%q) error = %v", key, err)
			continue
		}
		if provider != rt.Provider || modelID != rt.ModelID || reasoning != rt.Reasoning {
			t.Errorf("ParseRouteKey(%q) = %q/%q/%q, want %q/%q/%q",
				key, provider, modelID, reasoning, rt.Provider, rt.ModelID, rt.Reasoning)
		}
	}
}

func TestParseRouteKey_Errors(t *testing.T) {
	// Grammar error cases (B02 CONTRACTS §6): every one must wrap errValidation
	// and map to validation_failed with the exact message.
	type tc struct {
		in   string
		want string
	}
	cases := []tc{
		{"", `route key "": missing "/"`},
		{"claude", `route key "claude": missing "/"`},
		{"claude/opus", `route key "claude/opus": missing "@"`},
		{"/opus@high", `route key "/opus@high": empty provider`},
		{"claude/@high", `route key "claude/@high": empty model_id`},
		{"claude/opus@", `route key "claude/opus@": empty reasoning`},
		{"Claude/opus@high", `route key "Claude/opus@high": invalid provider "Claude"`},
		{"claude/op us@high", `route key "claude/op us@high": invalid model_id "op us"`},
		{"claude/opus@x", `route key "claude/opus@x": invalid reasoning "x"`},
		{"claude/opus@ultra", `route key "claude/opus@ultra": invalid reasoning "ultra"`},
	}
	for _, c := range cases {
		_, _, _, err := ParseRouteKey(c.in)
		if err == nil {
			t.Errorf("ParseRouteKey(%q) error = nil, want %q", c.in, c.want)
			continue
		}
		if !errors.Is(err, errValidation) {
			t.Errorf("ParseRouteKey(%q): err not wrapping errValidation: %v", c.in, err)
		}
		// The contract detail is the %w-suffix; the full error carries the
		// "validation failed: " sentinel prefix.
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("ParseRouteKey(%q) message = %q, want to contain %q", c.in, err.Error(), c.want)
		}
		if dto := toErrorDTO(err); dto.Code != "validation_failed" {
			t.Errorf("ParseRouteKey(%q) dto code = %q, want validation_failed", c.in, dto.Code)
		}
	}
}

func TestWeightConversion(t *testing.T) {
	// dtoWeights: rounds half-up to int, drops keys rounding to <=0.
	got0 := dtoWeights(map[string]decimal.Decimal{
		"a": decimal.NewFromFloat(2.5),
		"b": decimal.NewFromFloat(0.4),
		"c": decimal.NewFromFloat(1),
	})
	if got0["a"] != 3 {
		t.Errorf(`dtoWeights a = %d, want 3 (2.5 rounds half-up)`, got0["a"])
	}
	if _, ok := got0["b"]; ok {
		t.Error(`dtoWeights b present, want dropped (0.4 -> 0)`)
	}
	if got0["c"] != 1 {
		t.Errorf(`dtoWeights c = %d, want 1`, got0["c"])
	}

	// engineWeights: drops v<=0.
	got1, err := engineWeights(map[string]int{"a": 3, "b": 0, "c": -1})
	if err != nil {
		t.Fatalf("engineWeights error = %v", err)
	}
	if _, ok := got1["b"]; ok {
		t.Error(`engineWeights b present, want dropped (0)`)
	}
	if _, ok := got1["c"]; ok {
		t.Error(`engineWeights c present, want dropped (-1)`)
	}
	if got1["a"].String() != "3" {
		t.Errorf(`engineWeights a = %s, want 3`, got1["a"].String())
	}

	// engineWeights rejects v>5 with the exact message.
	_, err = engineWeights(map[string]int{"a": 6})
	if err == nil {
		t.Fatal("engineWeights({a:6}) error = nil, want errValidation")
	}
	if !errors.Is(err, errValidation) {
		t.Errorf("engineWeights({a:6}) err not wrapping errValidation: %v", err)
	}
	if !strings.Contains(err.Error(), `weight "a" is 6, must be 0..5`) {
		t.Errorf("engineWeights({a:6}) message = %q, want to contain %q", err.Error(), `weight "a" is 6, must be 0..5`)
	}

	// engineProfile: Tier1Share = CoreShare/100, Tier2Share = 1-Tier1Share.
	p, err := engineProfile(ProfileDetail{Slug: "s", CoreShare: 60, Tier1Weights: map[string]int{"intelligence": 4}})
	if err != nil {
		t.Fatalf("engineProfile error = %v", err)
	}
	if p.Name != "s" {
		t.Errorf("engineProfile Name = %q, want s", p.Name)
	}
	if p.Tier1Share.String() != "0.6" {
		t.Errorf("Tier1Share = %s, want 0.6", p.Tier1Share.String())
	}
	if p.Tier2Share.String() != "0.4" {
		t.Errorf("Tier2Share = %s, want 0.4", p.Tier2Share.String())
	}
	if w := p.Tier1Weights["intelligence"]; w.String() != "4" {
		t.Errorf("Tier1Weights[intelligence] = %s, want 4", w.String())
	}

	// round2: the documented 1.005 case -> 1.01 (half-up at 2dp).
	if got := round2(decimal.NewFromFloat(1.005)); got != 1.01 {
		t.Errorf("round2(1.005) = %v, want 1.01", got)
	}
	if got := round2(decimal.NewFromFloat(2.499)); got != 2.5 {
		t.Errorf("round2(2.499) = %v, want 2.5", got)
	}
}

func TestToErrorDTO(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		dto := toErrorDTO(nil)
		if dto.Code != "" || dto.Message != "" {
			t.Errorf("toErrorDTO(nil) = %+v, want empty", dto)
		}
	})
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"validation", fmt.Errorf("%w: bad slug", errValidation), "validation_failed"},
		{"builtin readonly", fmt.Errorf("%w: profile is builtin", errBuiltinReadonly), "builtin_readonly"},
		{"not found", fmt.Errorf("%w: no such profile", errNotFound), "not_found"},
		{"conflict", fmt.Errorf("%w: slug exists", errConflict), "conflict"},
		{"usage unavailable", fmt.Errorf("%w: fetch impossible", errUsageUnavailable), "usage_unavailable"},
		{"launch failed", fmt.Errorf("%w: spawn failed", errLaunchFailed), "launch_failed"},
		{"io error", errors.New("read config.toml: no such file"), "io_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if dto := toErrorDTO(tt.err); dto.Code != tt.code {
				t.Errorf("toErrorDTO code = %q, want %q", dto.Code, tt.code)
			}
		})
	}

	t.Run("scoresMissingError -> io_error", func(t *testing.T) {
		err := &scoresMissingError{Path: "/tmp/x/available_model_scores.csv"}
		dto := toErrorDTO(err)
		if dto.Code != "io_error" {
			t.Errorf("code = %q, want io_error", dto.Code)
		}
		if !strings.Contains(dto.Message, "run: which-model catalog refresh") {
			t.Errorf("message = %q, want to contain remedy", dto.Message)
		}
	})

	t.Run("ErrorDTO passes through", func(t *testing.T) {
		want := ErrorDTO{Code: "conflict", Message: "x"}
		if got := toErrorDTO(want); got != want {
			t.Errorf("value ErrorDTO pass-through = %+v, want %+v", got, want)
		}
		if got := toErrorDTO(&want); got != want {
			t.Errorf("ptr ErrorDTO pass-through = %+v, want %+v", got, want)
		}
	})

	t.Run("Error method", func(t *testing.T) {
		if got := (ErrorDTO{Code: "not_found", Message: "nope"}).Error(); got != "not_found: nope" {
			t.Errorf("Error() = %q, want %q", got, "not_found: nope")
		}
	})
}

func TestToErrorDTOExportPreservesBoundaryMapping(t *testing.T) {
	for _, code := range []string{"validation_failed", "not_found", "conflict"} {
		original := ErrorDTO{Code: code, Message: "synthetic"}
		if got := ToErrorDTO(original); got != original {
			t.Fatalf("mapped %+v to %+v", original, got)
		}
	}
	if got := ToErrorDTO(nil); got.Code != "" || got.Message != "" {
		t.Fatalf("nil=%+v", got)
	}
}
