package privacy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCompanyRetentionConcurrentAppend(t *testing.T) {
	c := testController()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			failures <- c.Append(path, Audit, []byte(fmt.Sprintf(`{"candidate":"codex:model-%d","evidence":{}}`, i)))
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	report, err := c.Prune(path, Audit, false)
	if err != nil || report.Retained != 12 {
		t.Fatalf("concurrent records lost: %+v %v", report, err)
	}
}

func TestCompanyRetentionCategoryDurations(t *testing.T) {
	for _, category := range []Category{Usage, History, Audit, Launch} {
		t.Run(string(category), func(t *testing.T) {
			c := testController()
			path := filepath.Join(t.TempDir(), "record.jsonl")
			var text string
			timestamp := c.Now().Add(-c.age(category) - time.Second).Format(time.RFC3339)
			if category == Usage {
				text = `{"fetched_at":"` + timestamp + `","snapshot":{"provider":"codex","windows":[]}}`
			} else {
				text = `{"ts":"` + timestamp + `","profile":"balanced","candidate":"codex:model","outcome":"started"}`
			}
			os.WriteFile(path, []byte(text), 0600)
			report, err := c.Prune(path, category, false)
			if err != nil || report.Removed != 1 || report.DeletedFiles != 1 {
				t.Fatalf("expiry=%+v %v", report, err)
			}
		})
	}
	c := testController()
	c.Policy.Policy.Retention.PickHistoryDays = 1
	if c.age(History) != 24*time.Hour {
		t.Fatal("administrator duration ignored")
	}
}

func TestCompanyMaintenanceScopesOwnedFiles(t *testing.T) {
	c := testController()
	base := t.TempDir()
	state, repo := filepath.Join(base, "state"), filepath.Join(base, "repo")
	os.MkdirAll(filepath.Join(state, "credentials"), 0700)
	os.MkdirAll(filepath.Join(repo, ".which-model"), 0700)
	credential := filepath.Join(state, "credentials", "copilot.json")
	config := filepath.Join(repo, ".which-model", "config.toml")
	os.WriteFile(credential, []byte("CREDENTIAL_CANARY"), 0600)
	os.WriteFile(config, []byte("CONFIG_CANARY"), 0600)
	os.WriteFile(filepath.Join(state, "launch.log"), []byte("RAW_OUTPUT_CANARY"), 0600)
	os.WriteFile(filepath.Join(repo, ".which-model", "evidence.jsonl"), []byte("IDENTITY_CANARY"), 0600)
	result, err := c.Maintain(Layout{StateDir: state, ProjectRoot: repo}, false, nil)
	if err != nil || result.Categories[Launch].DeletedFiles != 1 || result.Categories[Audit].DeletedFiles != 1 {
		t.Fatalf("maintenance=%+v %v", result, err)
	}
	for _, path := range []string{credential, config} {
		data, _ := os.ReadFile(path)
		if !strings.Contains(string(data), "CANARY") {
			t.Fatal("unselected credential/config changed")
		}
	}
	// A blocked data path produces a failure while other categories still complete.
	os.WriteFile(filepath.Join(state, "launch.log"), []byte("raw"), 0600)
	os.MkdirAll(filepath.Join(state, "pick", "history.jsonl"), 0700)
	result, err = c.Maintain(Layout{StateDir: state}, true, nil)
	if err == nil || result.Categories[History].Failed != 1 || result.Categories[Launch].DeletedFiles != 1 {
		t.Fatalf("partial failure=%+v %v", result, err)
	}
}
