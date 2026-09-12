package whichmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Exercise actual band calculation, managed persistence and history decoding;
// only the route/catalog and provider observation are synthetic.
func companyEvidencePick(t *testing.T, snapshot *usage.Snapshot, enabled bool) (string, HistoryEntry) {
	t.Helper()
	route := f26ClaudeRoute()
	route.WindowIDs = []string{"session", "weekly"}
	realBand := bandEvaluateFunc
	cfg, state := pickPipelineSetup(t, []routing.Route{route}, pickTwoScores(),
		func(bool, *config.Config) (bool, string) { return enabled, "flag" },
		func(context.Context, []string, pickFetchOptions) (map[string]*usageSnapshot, map[string]timeValue, error) {
			if !enabled {
				t.Fatal("score-only pick collected quota")
			}
			return map[string]*usageSnapshot{route.Provider: snapshot}, nil, nil
		}, realBand)
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	policy := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &policy}, nil }
	var stdout, stderr bytes.Buffer
	if err := RunPick(PickArgs{Profile: "complex_implementation", Strategy: "priority", ConfigPath: cfg, NoUsage: !enabled, JSON: true}, &stdout, &stderr); err != nil {
		t.Fatal(err, stderr.String())
	}
	records, err := readHistory(filepath.Join(state, "pick", "history.jsonl"))
	if err != nil || len(records) != 1 {
		t.Fatalf("history: %+v %v", records, err)
	}
	return cfg, records[0]
}

func TestCompanyPickEvidenceCoverageSurvivesStaleness(t *testing.T) {
	for _, scenario := range []string{"fresh-complete", "fresh-partial", "stale-partial", "aged-partial", "stale-unrelated", "stale-unknown", "stale-synthetic", "stale-uncomputable", "stale-complete"} {
		t.Run(scenario, func(t *testing.T) {
			percent := 10.0
			snapshot := &usage.Snapshot{Provider: "claude", UsageKnown: true, FetchedAt: time.Now(), Source: usage.SourceCache, Confidence: "cached", Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}}}
			complete := strings.HasSuffix(scenario, "-complete")
			if complete || scenario == "stale-uncomputable" {
				snapshot.Windows = append(snapshot.Windows, usage.Window{ID: "weekly", UsageKnown: true, UsedPercent: &percent})
			}
			wantState := advisory.Stale
			switch {
			case strings.HasPrefix(scenario, "fresh-"):
				wantState = advisory.Partial
				if complete {
					wantState = advisory.Current
				}
			case scenario == "aged-partial":
				snapshot.FetchedAt = time.Now().Add(-time.Hour)
			default:
				snapshot.Stale = true
			}
			switch scenario {
			case "stale-unrelated":
				snapshot.Windows[0].ID = "unrelated"
			case "stale-unknown":
				snapshot.UsageKnown = false
			case "stale-synthetic":
				snapshot.Windows[0].Synthetic = true
			case "stale-uncomputable":
				snapshot.Windows[1].UsedPercent = nil
			}
			_, record := companyEvidencePick(t, snapshot, true)
			if record.Evidence.QuotaState != wantState {
				t.Fatalf("quota state = %q, want %q", record.Evidence.QuotaState, wantState)
			}
			if complete {
				if record.Evidence.Band == nil || record.Evidence.Band.Name != "low" || record.Evidence.Band.UsedPercent != 10 {
					t.Fatalf("complete route band lost: %+v", record.Evidence.Band)
				}
			} else if record.Evidence.Band != nil {
				t.Fatalf("unconfirmed route pressure persisted as a numeric band: %+v", record.Evidence.Band)
			}
		})
	}
}

func TestCompanyPickEvidenceExplainText(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		name := "score-only"
		if enabled {
			name = "stale"
		}
		t.Run(name, func(t *testing.T) {
			percent := 10.0
			snapshot := &usage.Snapshot{Provider: "claude", UsageKnown: true, Stale: true, FetchedAt: time.Now().Add(-time.Hour), Source: usage.SourceCache, Confidence: "cached", Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}, {ID: "weekly", UsageKnown: true, UsedPercent: &percent}}}
			cfg, record := companyEvidencePick(t, snapshot, enabled)
			var stdout, stderr bytes.Buffer
			args := ExplainArgs{Last: true, ConfigPath: cfg}
			if err := RunExplain(args, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			wantState := advisory.Disabled
			if enabled {
				wantState = advisory.Stale
				if !strings.Contains(stdout.String(), "band: low (10% used, weight 1)") {
					t.Fatal("complete historical band missing from explanation")
				}
			}
			if record.Evidence.QuotaState != wantState || !strings.Contains(stdout.String(), advisory.ForState(wantState).Message) {
				t.Fatalf("recorded %s advisory missing:\n%s", wantState, stdout.String())
			}
			stdout.Reset()
			args.JSON = true
			if err := RunExplain(args, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			var result ExplainResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Evidence.QuotaState != wantState {
				t.Fatalf("JSON evidence changed: %s, %v", stdout.String(), err)
			}
		})
	}
}
