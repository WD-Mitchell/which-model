package config

import (
	"os"
	"path/filepath"
)

// AtomicWriteFile durably replaces path with data: MkdirAll(dir, 0o755), temp
// file in dir chmodded 0o600, write, fsync, close, rename over path, fsync
// dir. On error the temp file is removed and path is untouched. Promoted from
// pkg/whichmodel/config_cmd.go atomicWrite (B01 SPEC §2.8); the single write
// path for CLI `config set` and all service mutations.
func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	renamed = true
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}
