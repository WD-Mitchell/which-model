package publish

import (
	"encoding/json"
	"errors"
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

// jsonRoundTrip decodes v into out through encoding/json, mirroring F01's
// UnmarshalKey semantics (missing value = out untouched). Map keys are
// normalized like F01's TOML matching: case-insensitive with underscores
// ignored.
func jsonRoundTrip(v any, out any) error {
	data, err := json.Marshal(normalizeKeys(v))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func normalizeKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[strings.ToLower(strings.ReplaceAll(k, "_", ""))] = normalizeKeys(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeKeys(val)
		}
		return out
	default:
		return v
	}
}

// kv returns a fake returning v for key and the zero value (nil) for every
// other key.
func kv(key string, v any) fakeCfg {
	return fakeCfg{fn: func(k string, out any) error {
		if k != key {
			return jsonRoundTrip(nil, out)
		}
		return jsonRoundTrip(v, out)
	}}
}

func TestLoadMissingSection(t *testing.T) {
	pc, err := Load(kv("catalog.publish", nil))
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
	if pc.RawCSVPath != "available-model-data-export/available_model_raw_values.csv" {
		t.Errorf("raw artifact path = %q", pc.RawCSVPath)
	}
}

func TestLoadFullSection(t *testing.T) {
	cfg := fakeCfg{fn: func(key string, out any) error {
		switch key {
		case "catalog.publish":
			return jsonRoundTrip(map[string]any{
				"enabled":        true,
				"schedule":       "15 8 * * *",
				"timezone":       "America/New_York",
				"environment":    "csv-update",
				"branches":       []any{"main", "release"},
				"mode":           "pull-request",
				"auto_merge":     true,
				"merge_method":   "squash",
				"commit_message": "chore: custom commit",
				"pr_title":       "chore: custom PR",
				"pr_labels":      []any{"data", "automated"},
			}, out)
		case "catalog.raw_csv_path":
			return jsonRoundTrip("custom_raw.csv", out)
		default:
			return jsonRoundTrip(nil, out)
		}
	}}
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
			_, err := Load(kv("catalog.publish", tt.values))
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
	pc, err := Load(kv("catalog.publish", map[string]any{"pr_labels": []any{"data", "data", "x"}}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(pc.PRLabels, []string{"data", "x"}) {
		t.Errorf("PRLabels = %v, want [data x]", pc.PRLabels)
	}
}

func TestLoadDirectPushValuesUsed(t *testing.T) {
	pc, err := Load(kv("catalog.publish", map[string]any{"mode": "direct-push", "auto_merge": false}))
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
	pc, err := Load(kv("catalog.publish", map[string]any{"enabled": false}))
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
