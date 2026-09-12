package service

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/company"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompanyHistoryMinimizesOpaqueEvidence(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	path := filepath.Join(t.TempDir(), "pick", "history.jsonl")
	entry := PickHistoryEntry{TS: time.Now().UTC().Format(time.RFC3339), Profile: "balanced", CandidateID: "codex:model", Evidence: json.RawMessage(`{"profile":"balanced","account":"ACCOUNT_CANARY","raw_prompt":"PROMPT_CANARY"}`)}
	if err := AppendPick(path, entry); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "CANARY") {
		t.Fatal("opaque evidence persisted")
	}
	stats, _, err := AggregatePicks(path)
	if err != nil || stats["balanced"].Picks != 1 {
		t.Fatalf("retained stats: %+v %v", stats, err)
	}
	oldEntry := `{"ts":"` + time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339) + `","profile":"balanced","candidate_id":"codex:model"}`
	os.WriteFile(path, []byte(oldEntry+"\n"), 0600)
	stats, _, err = AggregatePicks(path)
	if err != nil || len(stats) != 0 {
		t.Fatal("expired history still counted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expired history not deleted")
	}
}
