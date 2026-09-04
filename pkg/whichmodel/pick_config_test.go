package whichmodel

import (
	"errors"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickInvalidCatalogConfigReturnsConfigError(t *testing.T) {
	path := pickTestConfig(t, t.TempDir(), "[catalog]\nscores_csv_path = 42\n")
	err, _, stderr := runPick(t, PickArgs{Profile: "balanced_implementation", ConfigPath: path, NoUsage: true, DryRun: true})
	var ce *config.ConfigError
	if !errors.As(err, &ce) || ce.ExitCode() != 2 {
		t.Fatalf("error=%T %v; want config exit 2", err, err)
	}
	if strings.Contains(stderr.String(), "score") {
		t.Fatalf("unexpected missing-score warning: %s", stderr)
	}
}

func TestPickUsesConfiguredScoresCSV(t *testing.T) {
	dir := t.TempDir()
	scores := filepath.Join(dir, "custom scores.csv")
	if err := os.WriteFile(scores, []byte("model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score\nModel,high,80,80,90,70,80,80\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := pickTestConfig(t, dir, pickScoresConfigBody(scores, "[catalog.publish]\nenabled = true\n"))
	setStateDir(t, func() string { return dir })
	setLoadRoutes(t, func(string) (routing.Table, error) {
		return routing.Table{Routes: []routing.Route{{Provider: "fixture", ModelID: "model", Model: "Model", Reasoning: "high", Provenance: routing.ProvenanceUserDeclared}}}, nil
	})
	err, out, stderr := runPick(t, PickArgs{Profile: "balanced_implementation", ConfigPath: path, NoUsage: true, DryRun: true})
	if err != nil {
		t.Fatalf("pick failed: %v; %s %s", err, out, stderr)
	}
	if !strings.Contains(out.String(), `"model_id":"model"`) && !strings.Contains(out.String(), `"model_id": "model"`) {
		t.Fatalf("missing selected route: %s", out)
	}
}
