// S05 CONTRACTS §6 — reconcile-logic test for the macOS LaunchAgent plist:
// enable → file exists with the exec path; enable again → idempotent; disable
// → gone; disable again → nil. Uses a temp dir standing in for the artifact
// location (writeLoginItem) and a plist golden byte-compare.
//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteLoginItemReconcile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "com.wdmitchell.which-model.plist")
	execPath := "/Application Support/which-model/bin/which-model"

	if err := writeLoginItem(true, execPath, path); err != nil {
		t.Fatalf("enable: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after enable: %v", err)
	}
	if !strings.Contains(string(data), execPath) {
		t.Fatalf("plist missing exec path %q:\n%s", execPath, data)
	}

	// Tacit (re)write is idempotent.
	if err := writeLoginItem(true, execPath, path); err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	// Disable removes.
	if err := writeLoginItem(false, execPath, path); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("artifact still present after disable: %v", err)
	}

	// Disable again → nil (removal when absent is success).
	if err := writeLoginItem(false, execPath, path); err != nil {
		t.Fatalf("disable-again: %v", err)
	}
}

func TestLoginItemPlistGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plist.golden")
	execPath := "/Applications/which-model.app/Contents/MacOS/which-model"

	if err := writeLoginItem(true, execPath, path); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// The template is CONTRACTS §5.1 verbatim; golden just pins the exact
	// exec-path substitution and Label.
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.wdmitchell.which-model</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
	if got := string(data); got != strings.ReplaceAll(want, "%s", execPath) {
		t.Fatalf("plist mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}