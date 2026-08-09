package whichmodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
)

func TestRenderProviders(t *testing.T) {
	t.Run("case 1-2-3: text exact, exclusions, multi excluded", func(t *testing.T) {
		configured := map[string][]string{
			"anthropic":      {},
			"github-copilot": {"grok-4.5"},
		}
		catalogue := []modelsdev.ProviderModel{
			{Provider: "anthropic", ModelID: "m1"}, {Provider: "anthropic", ModelID: "m2"},
			{Provider: "anthropic", ModelID: "m3"}, {Provider: "anthropic", ModelID: "m4"},
			{Provider: "anthropic", ModelID: "m5"}, {Provider: "anthropic", ModelID: "m6"},
			{Provider: "anthropic", ModelID: "m7"}, {Provider: "anthropic", ModelID: "m8"},
			{Provider: "anthropic", ModelID: "m9"}, {Provider: "anthropic", ModelID: "m10"},
			{Provider: "anthropic", ModelID: "m11"}, {Provider: "anthropic", ModelID: "m12"},
			{Provider: "github-copilot", ModelID: "c1"}, {Provider: "github-copilot", ModelID: "c2"},
			{Provider: "github-copilot", ModelID: "c3"}, {Provider: "github-copilot", ModelID: "grok-4.5"},
		}
		rows, err := renderProviders(configured, catalogue, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		var buf strings.Builder
		providersText(&buf, rows)
		got := buf.String()
		if !strings.Contains(got, "anthropic        12 models 0 excluded") {
			t.Errorf("text = %q, want anthropic row with 0 excluded", got)
		}
		if !strings.Contains(got, "github-copilot   3 models  1 excluded (grok-4.5)") {
			t.Errorf("text = %q, want github-copilot row", got)
		}
	})

	t.Run("case 3b: multi excluded formatting", func(t *testing.T) {
		configured := map[string][]string{"p": {"a", "b"}}
		catalogue := []modelsdev.ProviderModel{{Provider: "p", ModelID: "x"}}
		rows, err := renderProviders(configured, catalogue, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		var buf strings.Builder
		providersText(&buf, rows)
		if !strings.Contains(buf.String(), "2 excluded (a, b)") {
			t.Errorf("text = %q, want '2 excluded (a, b)'", buf.String())
		}
	})

	t.Run("case 4: sorted ids", func(t *testing.T) {
		configured := map[string][]string{"zebra": {}, "alpha": {}}
		rows, err := renderProviders(configured, nil, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		if len(rows) != 2 || rows[0].ID != "alpha" || rows[1].ID != "zebra" {
			t.Errorf("rows = %+v, want alpha before zebra", rows)
		}
	})

	t.Run("case 6: provider subset", func(t *testing.T) {
		configured := map[string][]string{"alpha": {}, "zebra": {}}
		rows, err := renderProviders(configured, nil, []string{"alpha"})
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		if len(rows) != 1 || rows[0].ID != "alpha" {
			t.Errorf("rows = %+v, want only alpha", rows)
		}
	})

	t.Run("case 9: missing from catalogue", func(t *testing.T) {
		configured := map[string][]string{"p": {}}
		rows, err := renderProviders(configured, nil, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		if len(rows) != 1 || len(rows[0].Models) != 0 {
			t.Errorf("rows = %+v, want 0 models", rows)
		}
	})

	t.Run("case 10: excluded filtering", func(t *testing.T) {
		configured := map[string][]string{"p": {"excluded-model"}}
		catalogue := []modelsdev.ProviderModel{
			{Provider: "p", ModelID: "kept"},
			{Provider: "p", ModelID: "excluded-model"},
		}
		rows, err := renderProviders(configured, catalogue, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		if len(rows[0].Models) != 1 || rows[0].Models[0].ModelID != "kept" {
			t.Errorf("Models = %+v, want only kept", rows[0].Models)
		}
		if len(rows[0].Excluded) != 1 || rows[0].Excluded[0] != "excluded-model" {
			t.Errorf("Excluded = %v, want [excluded-model]", rows[0].Excluded)
		}
	})

	t.Run("case 5-11: json shape with reasoning list", func(t *testing.T) {
		configured := map[string][]string{"p": {}}
		catalogue := []modelsdev.ProviderModel{
			{Provider: "p", ModelID: "m1", Name: "Model One", EffortLevels: []string{"max", "high"}},
		}
		rows, err := renderProviders(configured, catalogue, nil)
		if err != nil {
			t.Fatalf("renderProviders() error = %v", err)
		}
		doc := providersJSON(rows)
		entries, ok := doc["p"].([]map[string]any)
		if !ok || len(entries) != 1 {
			t.Fatalf("doc[p] = %+v", doc["p"])
		}
		reasoning, ok := entries[0]["reasoning"].([]string)
		if !ok || len(reasoning) != 2 || reasoning[0] != "max" || reasoning[1] != "high" {
			t.Errorf("reasoning = %+v, want [max, high]", entries[0]["reasoning"])
		}
	})
}

func TestProvidersCmd(t *testing.T) {
	t.Run("case 7: missing config", func(t *testing.T) {
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "providers", "--config", writeMinimalConfig(t, dir), "--provider-config", filepath.Join(dir, "missing.toml")})
		if code != 1 {
			t.Errorf("exit = %d, want 1; stderr=%s", code, stderr)
		}
		if !strings.Contains(stderr, "provider config not found at") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("case 8: missing cache via seam", func(t *testing.T) {
		orig := cacheReader
		defer func() { cacheReader = orig }()
		cacheReader = func(string) ([]modelsdev.ProviderModel, bool, error) {
			return nil, false, nil
		}
		dir := t.TempDir()
		code, _, stderr := captureExecuteFresh(t, []string{"catalog", "providers", "--config", writeMinimalConfig(t, dir), "--provider-config", writeEmptyProviders(t, dir)})
		if code != 1 {
			t.Errorf("exit = %d, want 1; stderr=%s", code, stderr)
		}
		if !strings.Contains(stderr, "provider catalogue not found at") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("json envelope smoke", func(t *testing.T) {
		orig := cacheReader
		defer func() { cacheReader = orig }()
		cacheReader = func(string) ([]modelsdev.ProviderModel, bool, error) {
			return []modelsdev.ProviderModel{{Provider: "anthropic", ModelID: "m1", Name: "M1"}}, true, nil
		}
		dir := t.TempDir()
		providersPath := filepath.Join(dir, "providers.toml")
		if err := os.WriteFile(providersPath, []byte("[providers.anthropic]\nexcluded_models=[]\n"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "providers", "--json", "--config", writeMinimalConfig(t, dir), "--provider-config", providersPath})
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("json.Unmarshal() error = %v; stdout=%s", err, stdout)
		}
		if doc["schema_version"] != "2.0" {
			t.Errorf("schema_version = %v, want 2.0", doc["schema_version"])
		}
		if _, ok := doc["anthropic"]; !ok {
			t.Errorf("doc = %+v, want anthropic key", doc)
		}
	})
}
