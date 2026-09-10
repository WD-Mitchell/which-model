package service

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyLaunchUsesStructuredRecords(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	policy := company.Snapshot{Managed: true, Policy: &p}
	readCompanyPolicy = func() (company.Snapshot, error) { return policy, nil }
	dir := t.TempDir()
	h := &HarnessService{s: &Services{paths: config.Paths{StateDir: dir}}}
	file, err := h.launchOutput(policy)
	if err != nil || file != nil {
		t.Fatalf("company launch captures child output: %v", err)
	}
	h.recordCompanyLaunch("codex", "codex", "model", "balanced", "started")
	data, err := os.ReadFile(filepath.Join(dir, "launch.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"outcome":"started"`) {
		t.Fatal("structured outcome absent")
	}
	if _, err := os.Stat(filepath.Join(dir, "launch.log")); !os.IsNotExist(err) {
		t.Fatal("raw child-output log created")
	}
	// Personal output behavior remains the existing appendable launch.log.
	file, err = h.launchOutput(company.Snapshot{})
	if err != nil || file == nil {
		t.Fatal("personal capture changed")
	}
	file.Close()
}
