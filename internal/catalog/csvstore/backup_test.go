package csvstore

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func bakNames(t *testing.T, dir, base string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".") && strings.HasSuffix(e.Name(), ".bak") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

func TestBackup(t *testing.T) {
	const content = "model,reasoning\nClaude Opus 5,max\n"

	t.Run("creates stamped backup with original bytes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		backupPath, err := Backup(path, DefaultBackupKeep)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(path)
		re := regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `\.\d{8}T\d{6}\.\d{6}Z(\.\d+)?\.bak$`)
		if !re.MatchString(filepath.Base(backupPath)) {
			t.Errorf("backup name %q does not match stamp pattern", backupPath)
		}
		got, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != content {
			t.Errorf("backup bytes = %q, want original", got)
		}
	})

	t.Run("collision suffix", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp := "20260807T214300000000Z"
		first, err := backupWithStamp(path, 5, stamp)
		if err != nil {
			t.Fatal(err)
		}
		second, err := backupWithStamp(path, 5, stamp)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(first, stamp+".bak") || strings.HasSuffix(first, ".1.bak") {
			t.Errorf("first = %q, want %s.bak", first, stamp)
		}
		if !strings.HasSuffix(second, stamp+".1.bak") {
			t.Errorf("second = %q, want %s.1.bak", second, stamp)
		}
		for _, p := range []string{first, second} {
			got, err := os.ReadFile(p)
			if err != nil || string(got) != content {
				t.Errorf("backup %q content mismatch: %v", p, err)
			}
		}
	})

	t.Run("rotation keep=5 removes oldest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		oldest := ""
		for i := range 6 {
			stamp := "2026010" + string(rune('1'+i)) + "T000000000000Z"
			if i == 0 {
				oldest = path + "." + stamp + ".bak"
			}
			if err := os.WriteFile(path+"."+stamp+".bak", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Backup(path, 5); err != nil {
			t.Fatal(err)
		}
		names := bakNames(t, dir, "data.csv")
		if len(names) != 5 {
			t.Fatalf("bak files = %d, want 5 (%v)", len(names), names)
		}
		if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("oldest backup %q still exists", oldest)
		}
	})

	t.Run("keep=1 leaves newest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, stamp := range []string{"20260101T000000000000Z", "20260102T000000000000Z", "20260103T000000000000Z"} {
			if err := os.WriteFile(path+"."+stamp+".bak", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Backup(path, 1); err != nil {
			t.Fatal(err)
		}
		names := bakNames(t, dir, "data.csv")
		if len(names) != 1 {
			t.Fatalf("bak files = %d, want 1 (%v)", len(names), names)
		}
	})

	t.Run("keep zero errors", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Backup(path, 0); err == nil {
			t.Fatal("expected error for keep=0")
		}
	})

	t.Run("missing target", func(t *testing.T) {
		_, err := Backup(filepath.Join(t.TempDir(), "absent.csv"), 5)
		if !errors.Is(err, ErrMissingFile) {
			t.Errorf("err = %v, want ErrMissingFile", err)
		}
	})

	t.Run("returned path exists after rotation", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		backupPath, err := Backup(path, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(backupPath); err != nil {
			t.Errorf("returned backup %q missing: %v", backupPath, err)
		}
	})

	t.Run("unrelated siblings untouched", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"other.csv", "data.txt", "data.bak"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Backup(path, 1); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"other.csv", "data.txt", "data.bak"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("sibling %q removed: %v", name, err)
			}
		}
	})

	t.Run("trailing newline preserved", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "data.csv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		backupPath, err := Backup(path, 5)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(backupPath)
		if string(got) != content {
			t.Errorf("backup bytes = %q, want identical", got)
		}
	})
}
