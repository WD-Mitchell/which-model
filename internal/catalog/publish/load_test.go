package publish

import (
	"errors"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

type fakeCfg struct {
	fn func(key string, out any) error
}

func (f fakeCfg) UnmarshalKey(key string, out any) error { return f.fn(key, out) }

// kv uses the real strict table-only decoder; scalar-permissive fakes hid #166.
func kv(t *testing.T, key string, value any) *config.Config {
	t.Helper()
	doc := map[string]any{}
	if value != nil {
		switch key {
		case "catalog.publish":
			doc["catalog"] = map[string]any{"publish": value}
		case "catalog":
			doc["catalog"] = value
		default:
			t.Fatalf("unsupported fixture key %q", key)
		}
	}
	data, err := toml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadMissingSection(t *testing.T) {
	pc, err := Load(kv(t, "catalog.publish", nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !pc.Enabled || !pc.AutoMerge {
		t.Errorf("bool defaults: %+v", pc)
	}
	if pc.Schedule != DefaultSchedule || pc.Timezone != DefaultTimezone ||
		pc.Mode != DefaultMode || pc.MergeMethod != DefaultMergeMethod ||
		pc.CommitMessage != DefaultCommitMessage || pc.PRTitle != DefaultPRTitle {
		t.Errorf("string defaults: %+v", pc)
	}
	if !reflect.DeepEqual(pc.Branches, DefaultBranches) {
		t.Errorf("Branches = %v", pc.Branches)
	}
	if !reflect.DeepEqual(pc.PRLabels, DefaultPRLabels) {
		t.Errorf("PRLabels = %v", pc.PRLabels)
	}
	if pc.RawCSVPath != "data/available_model_raw_values.csv" {
		t.Errorf("raw artifact path = %q", pc.RawCSVPath)
	}
}

func TestLoadFullSection(t *testing.T) {
	cfg := kv(t, "catalog", map[string]any{
		"raw_csv_path": "custom_raw.csv",
		"publish": map[string]any{
			"enabled": true, "schedule": "15 8 * * *", "timezone": "America/New_York", "environment": "csv-update", "branches": []string{"main", "release"}, "mode": "pull-request", "auto_merge": true, "merge_method": "squash", "commit_message": "chore: custom commit", "pr_title": "chore: custom PR", "pr_labels": []string{"data", "automated"},
		},
	})
	pc, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !pc.Enabled || pc.Schedule != "15 8 * * *" || pc.Timezone != "America/New_York" ||
		pc.Environment != "csv-update" ||
		!reflect.DeepEqual(pc.Branches, []string{"main", "release"}) || pc.Mode != "pull-request" ||
		!pc.AutoMerge || pc.MergeMethod != "squash" || pc.CommitMessage != "chore: custom commit" ||
		pc.PRTitle != "chore: custom PR" || !reflect.DeepEqual(pc.PRLabels, []string{"data", "automated"}) {
		t.Errorf("populated fields: %+v", pc)
	}
	if pc.RawCSVPath != "custom_raw.csv" {
		t.Errorf("raw artifact path = %q", pc.RawCSVPath)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]any
		wantSub string
	}{
		{"unknown mode", map[string]any{"mode": "rebasing"}, "catalog.publish.mode"},
		{"unknown merge method", map[string]any{"merge_method": "merge2"}, "merge_method"},
		{"empty branches", map[string]any{"branches": []any{}}, "branches must not be empty"},
		{"six-field cron", map[string]any{"schedule": "0 6 * * * *"}, "catalog.publish.schedule"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(kv(t, "catalog.publish", tt.values))
			if err == nil {
				t.Fatalf("Load() = nil, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("Load() error = %q, want substring %q", err, tt.wantSub)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.ExitCode() != 2 {
				t.Errorf("error = %T, want *ValidationError with ExitCode 2", err)
			}
		})
	}
}

func TestLoadUnmarshalErrorPropagated(t *testing.T) {
	sentinel := errors.New("sentinel decode failure")
	_, err := Load(fakeCfg{fn: func(key string, out any) error { return sentinel }})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Load() error = %v, want sentinel propagated", err)
	}
}

func TestLoadPRLabelsDeduplicated(t *testing.T) {
	pc, err := Load(kv(t, "catalog.publish", map[string]any{"pr_labels": []any{"data", "data", "x"}}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(pc.PRLabels, []string{"data", "x"}) {
		t.Errorf("PRLabels = %v, want [data x]", pc.PRLabels)
	}
}

func TestLoadDirectPushValuesUsed(t *testing.T) {
	pc, err := Load(kv(t, "catalog.publish", map[string]any{"mode": "direct-push", "auto_merge": false}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if pc.Mode != "direct-push" || pc.AutoMerge {
		t.Errorf("mode/auto_merge = %q/%v", pc.Mode, pc.AutoMerge)
	}
}

func TestLoadRealConfigSnakeCase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "which-model.toml")
	body := `[catalog.publish]
enabled = true
schedule = "0 6 * * *"
timezone = "Europe/London"
environment = "csv-update"
branches = ["main"]
mode = "direct-push"
auto_merge = false
merge_method = "squash"
commit_message = "chore(data): refresh available model scores"
pr_title = "chore(data): refresh available model scores"
pr_labels = ["data", "automated"]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := config.Default()
	if err := cfg.DecodeFile(path); err != nil {
		t.Fatalf("DecodeFile() error = %v", err)
	}
	pc, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if pc.Mode != "direct-push" || pc.AutoMerge || pc.Environment != "csv-update" {
		t.Errorf("mode/auto_merge/environment = %q/%v/%q, want direct-push/false/csv-update", pc.Mode, pc.AutoMerge, pc.Environment)
	}
}

func TestLoadEnabledFalse(t *testing.T) {
	pc, err := Load(kv(t, "catalog.publish", map[string]any{"enabled": false}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if pc.Enabled {
		t.Error("Enabled = true, want false")
	}
	if pc.Schedule != DefaultSchedule || pc.Timezone != DefaultTimezone || pc.Mode != DefaultMode ||
		pc.MergeMethod != DefaultMergeMethod || !pc.AutoMerge ||
		!reflect.DeepEqual(pc.Branches, DefaultBranches) || !reflect.DeepEqual(pc.PRLabels, DefaultPRLabels) {
		t.Errorf("other fields must keep defaults: %+v", pc)
	}
}

func TestLoadFullCatalogRealConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `[catalog]
raw_csv_path = "custom raw.csv"
scores_csv_path = "custom scores.csv"
cache_ttl = "12h"
warn_on_stale_scores = true
[catalog.publish]
enabled = true
branches = ["main"]
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if pc.RawCSVPath != "custom raw.csv" {
		t.Fatalf("raw path = %q", pc.RawCSVPath)
	}
}

func TestLoadPairedArtifactPaths(t *testing.T) {
	for _, same := range []bool{false, true} {
		raw := "custom raw.csv"
		scores := "custom scores.csv"
		if same {
			scores = "./custom raw.csv"
		}
		cfg := kv(t, "catalog", map[string]any{"raw_csv_path": raw, "scores_csv_path": scores})
		pc, err := Load(cfg)
		if same {
			if err == nil {
				t.Fatal("identical artifact paths accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if pc.RawCSVPath != raw || pc.ScoresCSVPath != scores {
			t.Fatalf("paths: %+v", pc)
		}
	}
	defaults, err := Load(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if defaults.ScoresCSVPath != "data/available_model_scores.csv" {
		t.Fatalf("default scores path = %q", defaults.ScoresCSVPath)
	}
}
