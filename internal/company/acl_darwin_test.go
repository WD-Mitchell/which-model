//go:build darwin

package company

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDarwinACLIsIndependentOfModeBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic-policy")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := checkExtendedACL(f); err != nil {
		t.Fatal("plain file rejected", err)
	}
	if out, err := exec.Command("/bin/chmod", "+a", "everyone allow write", path).CombinedOutput(); err != nil {
		t.Fatalf("fixture ACL: %v %s", err, out)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o022 != 0 {
		t.Fatal("fixture must retain safe mode bits")
	}
	if err := checkExtendedACL(f); err == nil {
		t.Fatal("ACL write grant was ignored")
	}
}
