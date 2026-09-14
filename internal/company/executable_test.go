package company

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApprovedInstallationDigestAndRedirect(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fixture.exe")
	header := []byte("MZ00")
	if runtime.GOOS == "linux" {
		header = []byte{0x7f, 'E', 'L', 'F'}
	}
	if runtime.GOOS == "darwin" {
		header = []byte{0xcf, 0xfa, 0xed, 0xfe}
	}
	data := append(header, []byte("synthetic native image fixture")...)
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	in := Installation{Path: path, SHA256: hex.EncodeToString(sum[:])}
	trust := func(*os.File, bool) error { return nil } // native ACL proof has its own elevated fixture
	if err := verifyInstallation(in, true, trust); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("MZaltered image"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := verifyInstallation(in, true, trust); err == nil {
		t.Fatal("changed digest accepted")
	}
	link := filepath.Join(dir, "redirect.exe")
	if os.Symlink(path, link) == nil {
		in.Path = link
		if err := verifyInstallation(in, true, trust); err == nil {
			t.Fatal("redirect accepted")
		}
	}
}
func TestApprovedInstallationRejectsScriptAndUserOwnedImage(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "script.exe")
	data := []byte("#!/bin/sh\nexit 0\n")
	os.WriteFile(path, data, 0755)
	sum := sha256.Sum256(data)
	in := Installation{Path: path, SHA256: hex.EncodeToString(sum[:])}
	if err := verifyInstallation(in, true, func(*os.File, bool) error { return nil }); err == nil {
		t.Fatal("script launcher accepted")
	}
	if os.Getuid() != 0 {
		if err := VerifyInstallation(in, true); err == nil {
			t.Fatal("unprotected image accepted")
		}
	}
}
