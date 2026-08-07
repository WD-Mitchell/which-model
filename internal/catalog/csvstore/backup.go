package csvstore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// backupWithStamp is the stamp-injectable Backup core (SPEC §2.4).
func backupWithStamp(path string, keep int, stamp string) (string, error) {
	if keep < 1 {
		return "", fmt.Errorf("keep must be at least 1")
	}
	content, _, err := security.ReadBoundedFile(path, MaxCsvBytes)
	if err != nil {
		return "", mapBoundedReadErr(err, path)
	}

	var backupPath string
	for n := 0; ; n++ {
		var candidate string
		if n == 0 {
			candidate = fmt.Sprintf("%s.%s.bak", path, stamp)
		} else {
			candidate = fmt.Sprintf("%s.%s.%d.bak", path, stamp, n)
		}
		f, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := f.Write(content); err != nil {
			f.Close()
			os.Remove(candidate)
			return "", err
		}
		if err := f.Sync(); err != nil {
			f.Close()
			os.Remove(candidate)
			return "", err
		}
		if err := f.Close(); err != nil {
			os.Remove(candidate)
			return "", err
		}
		backupPath = candidate
		break
	}

	// Rotation: keep the `keep` most recent siblings (newest-first by name;
	// fixed-width UTC stamps sort lexicographically).
	dir, base := filepath.Dir(path), filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, base+".") && strings.HasSuffix(name, ".bak") {
			names = append(names, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := keep; i < len(names); i++ {
		os.Remove(filepath.Join(dir, names[i])) // ignore "not exist"
	}
	return backupPath, nil
}

// Backup copies path to "<name>.<UTCstamp>.bak" (stamp format
// 20060102T150405.000000Z) with exclusive create and collision suffix ".1",
// ".2", …, fsyncs the copy, then rotates siblings matching "<name>.*.bak" to
// keep the `keep` most recent. keep < 1 → error. Returns the backup path.
// Errors: ErrMissingFile.
func Backup(path string, keep int) (backupPath string, err error) {
	return backupWithStamp(path, keep, time.Now().UTC().Format("20060102T150405.000000Z"))
}
