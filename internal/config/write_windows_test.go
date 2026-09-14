//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeWindowsConfigReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	for _, content := range []string{"[auth]\nuse_keychain=true\n", "[auth]\nuse_keychain=true\nnative_keychain=true\n"} {
		if err := AtomicWriteFile(path, []byte(content)); err != nil {
			t.Fatal("native replacement failed", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			t.Fatal("replacement did not commit expected configuration", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("staging files left behind", err)
	}
}
