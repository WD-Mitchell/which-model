package publish

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func singleBranchPC() *PublishConfig {
	pc := GoldenPC()
	pc.Branches = []string{"main"}
	return pc
}

func TestWriteFresh(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	summary, err := Write(pc, path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	want := `wrote ` + path + ` (schedule="0 6 * * *", branches=[main], mode=pull-request)`
	if summary != want {
		t.Errorf("summary = %q, want %q", summary, want)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	rendered, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if string(onDisk) != string(rendered) {
		t.Error("file bytes != Render(pc)")
	}
}

func TestWriteIdempotent(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(first) != string(second) {
		t.Error("second Write changed the file bytes")
	}
}

func TestWriteDisabledRemovesFile(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	pc.Enabled = false
	summary, err := Write(pc, path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if summary != "removed "+path+" (catalog.publish.enabled = false)" {
		t.Errorf("summary = %q", summary)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after disabled write (stat err = %v)", err)
	}
}

func TestWriteDisabledAbsent(t *testing.T) {
	pc := GoldenPC()
	pc.Enabled = false
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	summary, err := Write(pc, path)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if summary != "no workflow file present (catalog.publish.enabled = false)" {
		t.Errorf("summary = %q", summary)
	}
}

func TestCheckInSync(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Check(pc, path); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}
}

func TestCheckExtraNewline(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		t.Fatalf("append error = %v", err)
	}
	f.Close()

	err = Check(pc, path)
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("Check() = %T, want *DriftError", err)
	}
	if len(de.Lines) < 2 || !strings.HasPrefix(de.Lines[0], "--- ") || !strings.HasPrefix(de.Lines[1], "+++ ") {
		t.Errorf("Lines = %v, want ---/+++ headers", de.Lines)
	}
	if !strings.Contains(de.Error(), "+") {
		t.Errorf("Error() = %q, want a + in the diff", de.Error())
	}
}

func TestCheckMissingFile(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "refresh-model-data.yml")
	err := Check(pc, path)
	var de *DriftError
	if !errors.As(err, &de) {
		t.Fatalf("Check() = %T, want *DriftError", err)
	}
	if !strings.Contains(de.Error(), "is missing") {
		t.Errorf("Error() = %q, want is missing", de.Error())
	}
}

func TestCheckDisabled(t *testing.T) {
	pc := GoldenPC()
	pc.Enabled = false
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-model-data.yml")
	if err := Check(pc, path); err != nil {
		t.Fatalf("Check(disabled, absent) = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var de *DriftError
	if err := Check(pc, path); !errors.As(err, &de) {
		t.Fatalf("Check(disabled, stale) = %T, want *DriftError", err)
	}
}

func TestCheckCRLFDrift(t *testing.T) {
	pc := singleBranchPC()
	dir := t.TempDir()
	path := filepath.Join(dir, "refresh-model-data.yml")
	rendered, err := Render(pc)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	crlf := strings.ReplaceAll(string(rendered), "\n", "\r\n")
	if err := os.WriteFile(path, []byte(crlf), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var de *DriftError
	if err := Check(pc, path); !errors.As(err, &de) {
		t.Fatalf("Check(CRLF) = %T, want *DriftError (byte compare, no normalization)", err)
	}
}

func TestWriteNestedPath(t *testing.T) {
	pc := singleBranchPC()
	path := filepath.Join(t.TempDir(), "a", "b", "c.yml")
	if _, err := Write(pc, path); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created at nested path: %v", err)
	}
}
