package whichmodel

import (
	"bytes"
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"strings"
	"testing"
	"time"
)

func TestCompanyPickScoreOnlyIsLabelledWithoutChangingRank(t *testing.T) {
	cfg, _ := pickPipelineSetup(t, pickTwoRoutes(), pickTwoScores(), func(bool, *config.Config) (bool, string) { return false, "flag" }, nil, nil)
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	var out, stderr bytes.Buffer
	args := PickArgs{Profile: "complex_implementation", Strategy: "priority", NoUsage: true, ConfigPath: cfg, JSON: true}
	if err := RunPick(args, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	var res PickResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 || res.Candidates[0].ModelScore != 92 || res.UsageEnabled || !strings.Contains(out.String(), advisory.ForState(advisory.Disabled).Message) {
		t.Fatal(out.String())
	}
	out.Reset()
	args.JSON = false
	if err := RunPick(args, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Score-only recommendation; quota was not evaluated.") {
		t.Fatal(out.String())
	}
}
func TestCompanyPickEvidenceDoesNotCallCachedDataLive(t *testing.T) {
	now := time.Now()
	percent := 10.0
	c := Candidate{CandidateID: "codex:model", Route: RouteRef{Provider: "codex", ModelID: "model", WindowIDs: []string{"session"}}, Band: "available", BandWeight: 1}
	st := &runState{managed: true, usageEnabled: true, profile: "balanced", snapshots: map[string]*usageSnapshot{"codex": {Provider: "codex", UsageKnown: true, Source: usage.SourceCache, Confidence: "cached", FetchedAt: now, Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}}}}, lastVerified: map[string]timeValue{"codex": now}}
	ev := buildEvidence(st, &c, []ExcludedCandidate{})
	if ev.Confidence != "cached" || ev.LastVerified != "" || ev.QuotaState != "current" {
		t.Fatalf("cached evidence mislabelled: %+v", ev)
	}
	st.snapshots["codex"].Windows = nil
	ev = buildEvidence(st, &c, nil)
	if ev.Band != nil || ev.QuotaState != "unknown" {
		t.Fatalf("unknown allowance appears confirmed: %+v", ev)
	}
}
