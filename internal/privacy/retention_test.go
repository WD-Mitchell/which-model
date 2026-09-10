package privacy

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/company"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testController() Controller {
	p := company.Defaults()
	return Controller{Policy: company.Snapshot{Managed: true, Policy: &p}, Now: func() time.Time { return time.Date(2030, 1, 31, 12, 0, 0, 0, time.UTC) }}
}

func TestRetentionMinimizesWithoutRenewingClock(t *testing.T) {
	c := testController()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	old := c.Now().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	kept := c.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	records := `{"ts":"` + old + `","profile":"balanced","candidate_id":"codex:model","account":"OLD_IDENTITY_CANARY"}` + "\n" +
		`{"ts":"` + kept + `","profile":"balanced","candidate_id":"codex:model","account":"IDENTITY_CANARY","evidence":{"profile":"balanced","score_inputs":{"quality":1},"account":"PAYLOAD_CANARY","excluded_candidates":[{"reason_code":"provider_error","reason":"SECRET_ERROR_CANARY"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(records), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := c.Prune(path, History, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 || report.Scrubbed != 1 || report.Retained != 1 {
		t.Fatalf("report=%+v", report)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CANARY") || !strings.Contains(string(data), kept) {
		t.Fatal("identity survived or original recording time changed")
	}
	c.Now = func() time.Time { return time.Date(2030, 2, 22, 12, 0, 0, 0, time.UTC) }
	report, err = c.Prune(path, History, false)
	if err != nil || report.Removed != 1 {
		t.Fatalf("later expiry=%+v %v", report, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("empty expired store was not removed")
	}
}

func TestRetentionAppendAndManualPurge(t *testing.T) {
	c := testController()
	path := filepath.Join(t.TempDir(), "audit", "evidence.jsonl")
	input := []byte(`{"candidate":"codex:model","evidence":{"profile":"balanced","score_inputs":{"quality":1}},"raw_output":"SECRET_CANARY"}`)
	if err := c.Append(path, Audit, input); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil {
		t.Fatal("invalid record")
	}
	if record["ts"] != c.Now().Format(time.RFC3339) || strings.Contains(string(data), "CANARY") {
		t.Fatal("timestamp/minimization not enforced")
	}
	report, err := c.Prune(path, Audit, true)
	if err != nil || report.Removed != 1 {
		t.Fatalf("purge=%+v %v", report, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("purged file remains")
	}
}

func TestRetentionRefusesRedirectAndReportsFailure(t *testing.T) {
	c := testController()
	dir := t.TempDir()
	target := filepath.Join(dir, "provider-owned")
	if err := os.WriteFile(target, []byte("CREDENTIAL_CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "history.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	report, err := c.Prune(link, History, true)
	if err == nil || report.Failed != 1 {
		t.Fatalf("redirect=%+v %v", report, err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "CREDENTIAL_CANARY" {
		t.Fatal("provider-owned target changed")
	}
	if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), "CANARY") {
		t.Fatal("failure leaked data/path")
	}
}

func TestRetentionRejectsImmortalRecords(t *testing.T) {
	c := testController()
	for _, timestamp := range []string{"", "not-a-time", "2031-01-01T00:00:00Z"} {
		t.Run(timestamp, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.jsonl")
			data, _ := json.Marshal(map[string]any{"ts": timestamp, "profile": "balanced"})
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			report, err := c.Prune(path, History, false)
			if err != nil || report.Removed != 1 || report.DeletedFiles != 1 {
				t.Fatalf("invalid timestamp retained: %+v %v", report, err)
			}
		})
	}
}

func TestRetentionRefusesRedirectedParent(t *testing.T) {
	dir, target := t.TempDir(), t.TempDir()
	path := filepath.Join(target, "evidence.jsonl")
	if err := os.WriteFile(path, []byte("PROVIDER_CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, ".which-model")); err != nil {
		t.Skip("symlinks unavailable")
	}
	summary, err := testController().Maintain(Layout{ProjectRoot: dir}, true, []Category{Audit})
	if err == nil || summary.Categories[Audit].Failed == 0 {
		t.Fatal("redirected project directory accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "PROVIDER_CANARY" {
		t.Fatal("redirected target changed")
	}
}

func TestRetentionAdministratorIdentityOptOut(t *testing.T) {
	c := testController()
	c.Policy.Policy.IdentityFree = false
	path := filepath.Join(t.TempDir(), "codex.json")
	if err := c.WriteUsage(path, []byte(`{"snapshot":{"provider":"codex","account":"synthetic-account","plan":"synthetic-plan","raw_response":"PAYLOAD_CANARY","windows":[]}}`)); err != nil {
		t.Fatal(err)
	}
	data, err := c.Read(path, Usage)
	if err != nil || !strings.Contains(string(data), "synthetic-account") || strings.Contains(string(data), "CANARY") {
		t.Fatalf("administrator setting/payload whitelist failed: %v", err)
	}
}

func TestRetentionZeroDisablesPersistenceAndDeletesExisting(t *testing.T) {
	c := testController()
	c.Policy.Policy.Retention = company.Retention{}
	for _, category := range []Category{Usage, History, Audit, Launch} {
		path := filepath.Join(t.TempDir(), "record")
		if err := os.WriteFile(path, []byte("LEGACY_CANARY"), 0600); err != nil {
			t.Fatal(err)
		}
		var err error
		if category == Usage {
			err = c.WriteUsage(path, []byte(`{"snapshot":{"provider":"codex"}}`))
		} else {
			err = c.Append(path, category, []byte(`{"profile":"balanced","candidate":"codex:model","outcome":"started"}`))
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("zero-retention %s persisted", category)
		}
	}
	summary, err := c.Maintain(Layout{StateDir: t.TempDir()}, false, nil)
	if err != nil || len(summary.Categories) != 4 {
		t.Fatalf("zero retention rejected: %+v %v", summary, err)
	}
}
