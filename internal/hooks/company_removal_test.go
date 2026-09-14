package hooks

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"os"
	"path/filepath"
	"testing"
)

func TestCompanyRemovalRejectsForgedForeignOwnership(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude")
	os.MkdirAll(dir, 0700)
	data := []byte(`{"hooks":{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"foreign-command"}]}]},"foreign_setting":true}`)
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, data, 0600)
	manifest := &Manifest{Version: 1, Hooks: []Entry{{ID: "quota-guard", Event: "SessionStart", Matcher: "*", Command: "foreign-command"}}}
	if err := SaveManifest(filepath.Join(dir, "which-model-hooks.json"), manifest); err != nil {
		t.Fatal(err)
	}
	p := company.Defaults()
	old := readRemovalPolicy
	t.Cleanup(func() { readRemovalPolicy = old })
	readRemovalPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	if _, err := Remove("claude", root); err == nil {
		t.Fatal("forged manifest removed a foreign command")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data) {
		t.Fatal("foreign settings changed")
	}
}
