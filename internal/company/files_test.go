package company

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedReaderRejectsUserOwnedPolicyAndRedirects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1}`), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := readProtected(path); err == nil {
		t.Fatal("ordinary user/temp policy trusted")
	}
	link := filepath.Join(dir, "redirect")
	if err := os.Symlink(dir, link); err != nil {
		t.Log("symlink creation unavailable:", err)
		return
	}
	if _, err := readProtected(filepath.Join(link, "missing.json")); err == nil || os.IsNotExist(err) {
		t.Fatal("redirect mistaken for absent enrollment")
	}
}

// CI provisions this isolated fixture with native administrator ownership and
// safe parents. The environment key is read only by test code, never by Load.
func TestNativeProtectedEnrollment(t *testing.T) {
	dir := os.Getenv("COMPANY_POLICY_TEST_DIR")
	if dir == "" {
		t.Skip("native protected fixture not provisioned")
	}
	state, err := loadAt(filepath.Join(dir, "valid"), readProtected)
	if err != nil || !state.Managed || !state.Required {
		path := filepath.Join(dir, "valid", "required.json")
		for _, component := range append(parentPaths(path), path) {
			info, statErr := os.Lstat(component)
			if statErr != nil {
				t.Log(component, "lstat", statErr)
				continue
			}
			f, openErr := os.Open(component)
			if openErr != nil {
				t.Log(component, "open", openErr)
				continue
			}
			t.Logf("component=%s mode=%s sys=%T redirect=%v protection=%v", component, info.Mode(), info.Sys(), isRedirect(info), trustOpened(f, info.IsDir()))
			f.Close()
		}
		t.Fatal("native protected enrollment failed", err)
	}
	for _, name := range []string{"missing", "writable", "invalid", "redirect"} {
		if _, err := loadAt(filepath.Join(dir, name), readProtected); err == nil {
			t.Errorf("native %s fixture granted authority", name)
		}
	}
}
