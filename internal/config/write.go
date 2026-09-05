package config

import (
	"errors"
	"os"
	"path/filepath"
)

// AtomicWriteFile durably replaces path with data: MkdirAll(dir, 0o755), temp
// file in dir chmodded 0o600, write, fsync, close, rename over path, fsync
// dir. Pre-rename errors leave path untouched. Post-rename failures return
// CommittedWriteError: new bytes are visible but durability is unconfirmed. Promoted from
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
	if err := syncDirectory(dir); err != nil {
		return &CommittedWriteError{Err: err}
	}
	return nil
}

// CommittedWriteError means rename succeeded but directory sync failed.
type CommittedWriteError struct{ Err error }

func (e *CommittedWriteError) Error() string {
	return "config committed; directory sync failed: " + e.Err.Error()
}
func (e *CommittedWriteError) Unwrap() error { return e.Err }
func WriteCommitted(err error) bool {
	var committed *CommittedWriteError
	return errors.As(err, &committed)
}

var syncDirectory = func(dir string) error {
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
}
