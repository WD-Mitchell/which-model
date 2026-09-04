package config

import (
	"io"
	"os"
	"path/filepath"
)

// EnsureLegacyMigration moves a pre-platform-layout `~/.which-model` tree
// into the resolved per-OS locations (issue #39 review): the user config
// file, the whole legacy cache subtree, and the whole legacy state subtree
// (legacy cache/ -> CacheDir, state/ -> StateDir, config.toml ->
// UserConfigFile). When the canonical config file already exists the new
// layout wins and nothing is touched — the legacy tree is left for manual
// recovery. The legacy root is removed after a successful move. Safe to
// call on every start: a no-op without a legacy tree, and a completed
// migration removes it.
func EnsureLegacyMigration(paths Paths, home string) error {
	legacyRoot := filepath.Join(home, ".which-model")
	if info, err := os.Stat(legacyRoot); err != nil || !info.IsDir() {
		return nil
	}
	if _, err := os.Stat(paths.UserConfigFile); err == nil {
		return nil
	}

	for _, dir := range []string{filepath.Dir(paths.UserConfigFile), paths.CacheDir, paths.StateDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}

	legacyConfig := filepath.Join(legacyRoot, "config.toml")
	if fileRegular(legacyConfig) {
		if err := os.Rename(legacyConfig, paths.UserConfigFile); err != nil {
			return err
		}
	}
	for _, pair := range [][2]string{
		{filepath.Join(legacyRoot, "cache"), paths.CacheDir},
		{filepath.Join(legacyRoot, "state"), paths.StateDir},
	} {
		if info, err := os.Stat(pair[0]); err == nil && info.IsDir() {
			if err := mergeTree(pair[0], pair[1]); err != nil {
				return err
			}
			if err := os.RemoveAll(pair[0]); err != nil {
				return err
			}
		}
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(legacyRoot, entry.Name()), filepath.Join(paths.ConfigDir, ".legacy-"+entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(legacyRoot)
}

func fileRegular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// mergeTree recursively copies src into dst without overwriting existing
// files (first-wins: a file already present in the new tree stays).
func mergeTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if _, err := os.Lstat(target); err == nil {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
