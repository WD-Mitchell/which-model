package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	t.Run("creates missing parent dirs", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a", "b", "config.toml")
		if err := AtomicWriteFile(path, []byte("x = 1\n")); err != nil {
			t.Fatalf("AtomicWriteFile: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "x = 1\n" {
			t.Fatalf("content = %q", data)
		}
	})

	t.Run("file mode 0600", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := AtomicWriteFile(path, []byte("x = 1\n")); err != nil {
			t.Fatalf("AtomicWriteFile: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("mode = %v, want 0600", got)
		}
	})

	t.Run("overwrite replaces content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := AtomicWriteFile(path, []byte("first\n")); err != nil {
			t.Fatalf("AtomicWriteFile: %v", err)
		}
		if err := AtomicWriteFile(path, []byte("second\n")); err != nil {
			t.Fatalf("AtomicWriteFile: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "second\n" {
			t.Fatalf("content = %q", data)
		}
	})

	t.Run("failure leaves destination untouched and no temp litter", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root; directory permissions are not enforced")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		if err := AtomicWriteFile(path, []byte("keep me\n")); err != nil {
			t.Fatalf("AtomicWriteFile: %v", err)
		}
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		if err := AtomicWriteFile(path, []byte("clobber\n")); err == nil {
			t.Fatal("AtomicWriteFile: want error on unwritable dir")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "keep me\n" {
			t.Fatalf("content = %q, want untouched original", data)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 1 || entries[0].Name() != "config.toml" {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("dir entries = %v, want only config.toml", names)
		}
	})
}
