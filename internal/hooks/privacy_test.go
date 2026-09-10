package hooks

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/company"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyAuditDropsPayloadAndUsesCentralStore(t *testing.T) {
	p := company.Defaults()
	policy := company.Snapshot{Managed: true, Policy: &p}
	state, repo := t.TempDir(), t.TempDir()
	legacy := filepath.Join(repo, ".which-model")
	os.MkdirAll(legacy, 0700)
	os.WriteFile(filepath.Join(legacy, "evidence.jsonl"), []byte("LEGACY_CANARY"), 0600)
	var doc auditDocument
	if err := json.Unmarshal([]byte(`{"schema_version":"2.0","candidate":"codex:model","evidence":{"profile":"balanced","score_inputs":{"quality":1},"route_provenance":"provider_live","excluded_candidates":[{"route":{"provider":"claude","model_id":"model","model":"Claude Model","reasoning":"default","window_ids":[]},"reason_code":"provider_error","reason":"SECRET_ERROR_CANARY"}]}}`), &doc); err != nil {
		t.Fatal(err)
	}
	out, err := companyAudit(policy, doc, Options{RepoRoot: repo, companyStateDir: state, Env: map[string]string{"WHICH_MODEL_DISPATCHED_MODEL": "other-model"}})
	if err != nil || !strings.Contains(string(out), "dispatch evidence recorded") {
		t.Fatalf("audit: %s %v", out, err)
	}
	if strings.Contains(string(out), state) || strings.Contains(string(out), repo) {
		t.Fatal("audit status includes identity-bearing path")
	}
	for _, file := range []string{"evidence.jsonl", "mismatches.jsonl"} {
		data, err := os.ReadFile(filepath.Join(state, "audit", file))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "CANARY") || !strings.Contains(string(data), `"ts"`) {
			t.Fatal("audit payload/timestamp policy failed")
		}
	}
	if _, err := os.Stat(filepath.Join(legacy, "evidence.jsonl")); !os.IsNotExist(err) {
		t.Fatal("legacy project audit not removed")
	}
}

func TestCompanyAuditFailureIsVisibleAndAdvisory(t *testing.T) {
	p := company.Defaults()
	state := filepath.Join(t.TempDir(), "not-a-directory")
	os.WriteFile(state, []byte("SYNTHETIC_CANARY"), 0600)
	out, err := companyAudit(company.Snapshot{Managed: true, Policy: &p}, auditDocument{Candidate: "codex:model"}, Options{companyStateDir: state})
	if err != nil || !strings.Contains(string(out), `"audit_recorded":false`) || !strings.Contains(string(out), `"decision":"approve"`) {
		t.Fatalf("failure: %s %v", out, err)
	}
	if strings.Contains(string(out), "CANARY") || strings.Contains(string(out), state) {
		t.Fatal("failure leaked payload/path")
	}
}
