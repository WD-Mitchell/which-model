package skills

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"os"
	"path/filepath"
	"testing"
)

func TestCompanyRemovalPreservesModifiedAndForeignFiles(t *testing.T) {
	root, _ := fakeRepo(t)
	if _, err := Install("model-selection", TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".agents", "skills", "model-selection")
	modified := filepath.Join(dir, "SKILL.md")
	foreign := filepath.Join(dir, "foreign.txt")
	os.WriteFile(modified, []byte("foreign edit"), 0600)
	os.WriteFile(foreign, []byte("foreign file"), 0600)
	p := company.Defaults()
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	if _, err := Remove("model-selection", TargetGeneric, false, true); err == nil {
		t.Fatal("managed force removed modified file")
	}
	for path, want := range map[string]string{modified: "foreign edit", foreign: "foreign file"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatal("foreign data changed")
		}
	}
}
