package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/approvedexec"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestCompanyLaunchEvidenceAndAuditFailuresRemainAdvisory(t *testing.T) {
	for _, scenario := range []string{"current", "critical", "missing", "unknown", "partial", "stale", "disabled", "authentication", "provider", "pre-audit", "post-audit", "history", "start-failure", "copy"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := "[usage]\nenabled='true'\nbackend='native'\n[providers.codex]\nenabled=true\n"
			if scenario == "copy" {
				cfg += "[gui]\ncopy_command_instead=true\n"
			}
			if scenario == "disabled" {
				cfg = strings.Replace(cfg, "enabled='true'", "enabled='false'", 1)
			}
			svc, _ := newTestServices(t, WithConfigTOML(cfg))
			svc.routes = routing.Table{Routes: []routing.Route{{Provider: "codex", ModelID: "model", Reasoning: "high", WindowIDs: []string{"session"}}}}
			percent := 25.0
			snap := usage.Snapshot{Provider: "codex", FetchedAt: time.Now(), UsageKnown: true, Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}}}
			if scenario == "unknown" {
				snap.UsageKnown = false
			}
			if scenario == "partial" {
				svc.routes.Routes[0].WindowIDs = []string{"session", "weekly"}
			}
			if scenario == "critical" {
				percent = 99
			}
			if scenario == "stale" {
				snap.Stale = true
			}
			if scenario == "authentication" {
				snap.Failure = &usage.Failure{Code: "unauthorized", Message: "SECRET_CANARY"}
			}
			if scenario == "provider" {
				snap.Failure = &usage.Failure{Code: "network", Message: "SECRET_CANARY"}
			}
			svc.usageObservations = map[string]usage.Snapshot{"codex": snap}
			if scenario == "missing" {
				svc.usageObservations = nil
			}
			p := company.Defaults()
			p.AllowedProviders = []string{"codex"}
			p.Executables = []company.Executable{{ID: "codex", Path: "/approved/image", Args: []string{"{model_id}"}}}
			oldPolicy, oldVerify, oldStart, oldAudit := readCompanyPolicy, verifyCompanyPlan, startCompanyProcess, writeLaunchAudit
			t.Cleanup(func() {
				readCompanyPolicy, verifyCompanyPlan, startCompanyProcess, writeLaunchAudit = oldPolicy, oldVerify, oldStart, oldAudit
			})
			readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
			verifyCompanyPlan = func(approvedexec.Plan) error { return nil }
			phases := []string{}
			started := false
			writeLaunchAudit = func(_ string, _ company.Snapshot, record launchAuditRecord) error {
				phases = append(phases, record.Phase)
				if (scenario == "pre-audit" && record.Phase == "launch_intent") || (scenario == "post-audit" && record.Phase == "launch_started") {
					return errors.New("SECRET_CANARY")
				}
				return nil
			}
			startCompanyProcess = func(*exec.Cmd) error {
				started = true
				if scenario == "start-failure" {
					return errors.New("SECRET_CANARY")
				}
				return nil
			}
			if scenario == "history" {
				svc.recordPick = func(context.Context, string, string) error { return errors.New("SECRET_CANARY") }
			}
			result, err := svc.Harnesses().Launch(context.Background(), "codex", "codex/model@high", "balanced")
			if scenario == "copy" {
				if started || err != nil || !result.Copied || len(phases) != 1 || phases[0] != "copy_prepared" {
					t.Fatalf("copy misreported execution: %+v %v %v", result, phases, err)
				}
				return
			}
			if !started {
				t.Fatal("evidence failure prevented process attempt")
			}
			if scenario == "start-failure" {
				if err == nil || strings.Contains(err.Error(), "CANARY") || phases[len(phases)-1] != "launch_failed" {
					t.Fatalf("start failure misreported: %+v %v", phases, err)
				}
				return
			}
			if err != nil || result.Copied || (len(result.Advisories) == 0 && scenario != "current" && scenario != "critical") || phases[0] != "launch_intent" || phases[len(phases)-1] != "launch_started" {
				t.Fatalf("advisory outcome: %+v phases=%v err=%v", result, phases, err)
			}
			for _, message := range result.Advisories {
				if strings.Contains(message, "CANARY") {
					t.Fatal("diagnostic payload escaped")
				}
			}
		})
	}
}
func TestCompanySelectedRouteKeepsIndependentEvidence(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nenabled='true'\nbackend='native'\n[providers.codex]\nenabled=true\n[providers.claude]\nenabled=true\n"))
	percent := 5.0
	svc.usageObservations = map[string]usage.Snapshot{"codex": {FetchedAt: time.Now(), UsageKnown: true, Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}}}}
	for provider, want := range map[string]string{"codex": advisory.Current, "claude": advisory.Missing} {
		report := svc.routeEvidenceLocked(routing.Route{Provider: provider, WindowIDs: []string{"session"}}, time.Now())
		if report.State != want {
			t.Fatalf("%s borrowed evidence: %+v", provider, report)
		}
	}
}

func TestCompanyRankLabelsWithoutChangingScores(t *testing.T) {
	svc, _ := fixtureServices(t)
	before, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
	if err != nil {
		t.Fatal(err)
	}
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	after, err := svc.Rank(context.Background(), RankRequest{ProfileSlug: "test_profile", Holds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if after.RecommendationMode != "score_only" {
		t.Fatal("missing score-only label")
	}
	after.RecommendationMode = ""
	for i := range after.Candidates {
		if after.Candidates[i].QuotaEvidence == nil {
			t.Fatal("missing route evidence")
		}
		after.Candidates[i].QuotaEvidence = nil
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("advisory changed ranking: %+v != %+v", before, after)
	}
}
func TestCompanyLaunchAuditCorrelatesActualPhases(t *testing.T) {
	p := company.Defaults()
	policy := company.Snapshot{Managed: true, Policy: &p}
	dir := t.TempDir()
	record := newLaunchAudit("codex", "model", "balanced", advisory.ForState(advisory.Missing))
	for _, phase := range []string{"launch_intent", "launch_started"} {
		record.Phase = phase
		if err := writeLaunchAudit(dir, policy, record); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "audit", "evidence.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatal(string(data))
	}
	for i, line := range lines {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatal(err)
		}
		if got["launch_id"] != record.LaunchID || got["quota_state"] != "missing" || got["phase"] != []string{"launch_intent", "launch_started"}[i] {
			t.Fatal(got)
		}
	}
	p.Retention.AuditRecordsDays = 0
	if err := writeLaunchAudit(dir, policy, record); err == nil {
		t.Fatal("disabled retention reported audit success")
	}
}
