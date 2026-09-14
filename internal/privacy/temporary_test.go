package privacy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func temporaryFixture(t *testing.T, dir, pattern string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("UNCOMMITTED_IDENTITY_CANARY"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestCompanyRetentionAbandonedWrites(t *testing.T) {
	for _, category := range []Category{Usage, History, Audit, Launch} {
		for _, purge := range []bool{false, true} {
			name := string(category) + "/cleanup"
			if purge {
				name = string(category) + "/purge"
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				layout := Layout{StateDir: filepath.Join(root, "state"), CacheDirs: []string{filepath.Join(root, "cache")}}
				path := map[Category]string{
					Usage:   filepath.Join(layout.CacheDirs[0], "codex.json"),
					History: filepath.Join(layout.StateDir, "pick", "history.jsonl"),
					Audit:   filepath.Join(layout.StateDir, "audit", "evidence.jsonl"),
					Launch:  filepath.Join(layout.StateDir, "launch.jsonl"),
				}[category]
				// Match AtomicWriteFile's owned namespace, including a missing final
				// file and incomplete JSON left before rename after an interruption.
				leftover := temporaryFixture(t, filepath.Dir(path), "."+filepath.Base(path)+".")
				backup := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".backup")
				if err := os.WriteFile(backup, []byte("USER_BACKUP_CANARY"), 0600); err != nil {
					t.Fatal(err)
				}
				result, err := testController().Maintain(layout, purge, nil)
				if err != nil || result.Categories[category].DeletedFiles != 1 || result.Categories[category].Failed != 0 {
					t.Fatalf("temporary cleanup=%+v %v", result, err)
				}
				if _, err := os.Stat(leftover); !os.IsNotExist(err) {
					t.Fatal("uncommitted product data remains", err)
				}
				if data, err := os.ReadFile(backup); err != nil || string(data) != "USER_BACKUP_CANARY" {
					t.Fatal("unrelated backup changed", err)
				}
			})
		}
	}
}

func TestCompanyRetentionLegacyTemporaryCache(t *testing.T) {
	dir := t.TempDir()
	for _, pattern := range []string{".tmp-codex-*", ".tmp-claude-*", ".codex.json."} {
		temporaryFixture(t, dir, pattern)
	}
	result, err := testController().Maintain(Layout{CacheDirs: []string{dir}}, false, []Category{Usage})
	if err != nil || result.Categories[Usage].DeletedFiles != 3 {
		t.Fatalf("legacy cleanup=%+v %v", result, err)
	}
}

func TestCompanyRetentionTemporaryWriterLock(t *testing.T) {
	c := testController()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	unlock, err := lockOwned(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if unlock != nil {
			unlock()
		}
	}()
	tmp := temporaryFixture(t, filepath.Dir(path), ".history.jsonl.")
	data := []byte(`{"ts":"` + c.Now().Format(time.RFC3339) + `","profile":"balanced","candidate_id":"codex:model"}`)
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		report Report
		err    error
	}
	done := make(chan outcome, 1)
	go func() { report, err := c.Prune(path, History, false); done <- outcome{report, err} }()
	select {
	case result := <-done:
		t.Fatalf("maintenance did not wait for the active writer: %+v", result)
	case <-time.After(75 * time.Millisecond):
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal("active writer's temporary file was removed", err)
	}
	unlock()
	unlock = nil
	select {
	case result := <-done:
		if result.err != nil || result.report.Retained != 1 || result.report.DeletedFiles != 0 {
			t.Fatalf("committed record not preserved: %+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("maintenance did not resume after the writer released its lock")
	}
}

func TestCompanyRetentionUnsafeTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "provider-owned")
	if err := os.WriteFile(target, []byte("CREDENTIAL_CANARY"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, ".codex.json.123")); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".codex.json.456"), 0700); err != nil {
		t.Fatal(err)
	}
	temporaryFixture(t, dir, ".codex.json.")
	result, err := testController().Maintain(Layout{CacheDirs: []string{dir}}, true, []Category{Usage})
	if err == nil || result.Categories[Usage].Failed != 2 || result.Categories[Usage].DeletedFiles != 1 {
		t.Fatalf("partial cleanup=%+v %v", result, err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "CREDENTIAL_CANARY" {
		t.Fatal("redirected credential changed", err)
	}
}

func TestCompanyRetentionTemporaryInventoryBound(t *testing.T) {
	dir := t.TempDir()
	leftover := temporaryFixture(t, dir, ".history.jsonl.")
	for i := 0; i < 1024; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("unrelated-%d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := testController().Prune(filepath.Join(dir, "history.jsonl"), History, true)
	if err == nil || report.Failed != 1 || report.DeletedFiles != 0 {
		t.Fatalf("overfull directory claimed complete cleanup: %+v %v", report, err)
	}
	if _, err := os.Stat(leftover); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyRetentionTemporaryZeroAndWrite(t *testing.T) {
	for _, zero := range []bool{false, true} {
		for _, category := range []Category{Usage, History, Audit, Launch} {
			t.Run(fmt.Sprintf("%s/zero=%v", category, zero), func(t *testing.T) {
				c := testController()
				if zero {
					c.Policy.Policy.Retention.UsageSnapshotsHours = 0
					c.Policy.Policy.Retention.PickHistoryDays = 0
					c.Policy.Policy.Retention.AuditRecordsDays = 0
					c.Policy.Policy.Retention.LaunchLogsDays = 0
				}
				path := filepath.Join(t.TempDir(), "record.jsonl")
				leftover := temporaryFixture(t, filepath.Dir(path), ".record.jsonl.")
				var err error
				if category == Usage {
					err = c.WriteUsage(path, []byte(`{"snapshot":{"provider":"codex","windows":[]}}`))
				} else {
					err = c.Append(path, category, []byte(`{"profile":"balanced","candidate":"codex:model","outcome":"started"}`))
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(leftover); !os.IsNotExist(err) {
					t.Fatal("old temporary record remains", err)
				}
				_, err = os.Stat(path)
				if (zero && !os.IsNotExist(err)) || (!zero && err != nil) {
					t.Fatal("unexpected persistence outcome", err)
				}
			})
		}
	}
}
