// Package privacy applies company retention to explicitly owned product records.
// It never discovers credentials, provider-owned files or arbitrary repositories.
package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/gofrs/flock"
)

type Category string

const (
	Usage   Category = "usage_snapshots"
	History Category = "pick_history"
	Audit   Category = "audit_records"
	Launch  Category = "launch_logs"
)

func validCategory(category Category) bool {
	return category == Usage || category == History || category == Audit || category == Launch
}

type Report struct {
	Retained     int `json:"retained_records"`
	Removed      int `json:"removed_records"`
	Scrubbed     int `json:"scrubbed_records"`
	DeletedFiles int `json:"deleted_files"`
	Failed       int `json:"failed_files"`
}

type Error struct {
	Category  Category
	Operation string
}

func (e *Error) Error() string {
	return "company privacy " + string(e.Category) + ": " + e.Operation + " failed; cleanup is incomplete"
}

type Controller struct {
	// Callers supply only the independently loaded protected company snapshot.
	Policy company.Snapshot
	Now    func() time.Time
}

func Current() (Controller, error) { p, err := company.Load(); return Controller{Policy: p}, err }
func (c Controller) Enabled() bool { return c.Policy.Managed && c.Policy.Policy != nil }
func (c Controller) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}
func (c Controller) age(category Category) time.Duration {
	if !c.Enabled() {
		return 0
	}
	r := c.Policy.Policy.Retention
	switch category {
	case Usage:
		return time.Duration(r.UsageSnapshotsHours) * time.Hour
	case History:
		return time.Duration(r.PickHistoryDays) * 24 * time.Hour
	case Audit:
		return time.Duration(r.AuditRecordsDays) * 24 * time.Hour
	case Launch:
		return time.Duration(r.LaunchLogsDays) * 24 * time.Hour
	default:
		return 0
	}
}

const maxStoreBytes = 64 << 20
const maxRecordBytes = 4 << 20

// readOwned checks the leaf and opened identity before reading any content.
// Native OS permissions remain authoritative; a redirected leaf is never read.
func readOwned(path string, limit int64) ([]byte, bool, error) {
	if err := safeParents(path); err != nil {
		return nil, false, err
	}
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, false, errors.New("unsafe entry")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, false, errors.New("changed entry")
	}
	if before.Size() > limit {
		return nil, true, nil
	} // oversized owned data is discarded without reading it
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return nil, true, nil
	}
	return data, true, nil
}

func lockOwned(path string, create bool) (func(), error) {
	if err := safeParents(path); err != nil {
		return nil, err
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	lockPath := path + ".privacy.lock"
	if info, err := os.Lstat(lockPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("unsafe lock")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l := flock.New(lockPath, flock.SetPermissions(0600))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ok, err := l.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil || !ok {
		l.Close()
		return nil, errors.New("lock unavailable")
	}
	return func() { _ = l.Unlock(); _ = l.Close() }, nil
}

func (c Controller) filter(category Category, data []byte, purge bool) ([]byte, Report) {
	var report Report
	var output bytes.Buffer
	for len(data) > 0 {
		var line []byte
		if category == Usage {
			line, data = data, nil
		} else {
			line, data, _ = bytes.Cut(data, []byte{'\n'})
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if purge || len(line) > maxRecordBytes {
			report.Removed++
			continue
		}
		var original map[string]any
		if json.Unmarshal(line, &original) != nil || original == nil {
			report.Removed++
			continue
		}
		timeKey := "ts"
		if category == Usage {
			timeKey = "fetched_at"
		}
		rawTime, _ := original[timeKey].(string)
		created, err := time.Parse(time.RFC3339Nano, rawTime)
		now := c.now()
		if err != nil || created.IsZero() || created.After(now) || !created.Add(c.age(category)).After(now) {
			report.Removed++
			continue
		}
		minimized, ok := minimize(category, original, c.Policy.Policy.IdentityFree)
		if !ok {
			report.Removed++
			continue
		}
		minimized[timeKey] = rawTime // never renew the retention clock during migration
		minimized["privacy_version"] = 1
		encoded, err := json.Marshal(minimized)
		if err != nil {
			report.Removed++
			continue
		}
		var compact bytes.Buffer
		_ = json.Compact(&compact, line)
		if !bytes.Equal(compact.Bytes(), encoded) {
			report.Scrubbed++
		}
		output.Write(encoded)
		output.WriteByte('\n')
		report.Retained++
	}
	return output.Bytes(), report
}

func (c Controller) Prune(path string, category Category, purge bool) (Report, error) {
	_, report, err := c.maintain(path, category, purge)
	return report, err
}

// Read returns only the minimized retained data from the same locked transaction
// that performs migration/deletion; it never reopens an unchecked cache value.
func (c Controller) Read(path string, category Category) ([]byte, error) {
	data, _, err := c.maintain(path, category, false)
	return data, err
}

func (c Controller) maintain(path string, category Category, purge bool) ([]byte, Report, error) {
	if !c.Enabled() {
		return nil, Report{}, nil
	}
	var temporaryReport Report
	failure := func(operation string) ([]byte, Report, error) {
		return nil, addReport(temporaryReport, Report{Failed: 1}), &Error{category, operation}
	}
	if !validCategory(category) || c.age(category) < 0 {
		return failure("policy")
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		files, err := temporaryFiles(path, category)
		if err != nil {
			return failure("temporary inspection")
		}
		if len(files) == 0 {
			return nil, Report{}, nil
		}
	}
	unlock, err := lockOwned(path, false)
	if err != nil {
		return failure("lock")
	}
	defer unlock()
	temporaryReport, err = c.pruneTemporaryFiles(path, category)
	if err != nil {
		return nil, temporaryReport, err
	}
	limit := int64(maxStoreBytes)
	if category == Usage {
		limit = maxRecordBytes
	}
	data, exists, err := readOwned(path, limit)
	if err != nil {
		return failure("read")
	}
	if !exists {
		return nil, temporaryReport, nil
	}
	kept, report := c.filter(category, data, purge)
	if len(kept) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return failure("delete")
		}
		report.DeletedFiles = 1
	} else if report.Removed > 0 || report.Scrubbed > 0 {
		if err := config.AtomicWriteFile(path, kept); err != nil {
			return failure("rewrite")
		}
	}
	return kept, addReport(temporaryReport, report), nil
}

// WriteUsage applies the same payload policy to live and delegated snapshots.
// The outer recording time is supplied here, independently of provider timestamps.
func (c Controller) WriteUsage(path string, data []byte) error {
	if c.Enabled() && c.age(Usage) == 0 {
		_, err := c.Prune(path, Usage, true)
		return err
	}
	if !c.Enabled() || len(data) > maxRecordBytes {
		return &Error{Usage, "record"}
	}
	var input map[string]any
	if json.Unmarshal(data, &input) != nil {
		return &Error{Usage, "record"}
	}
	record, ok := minimize(Usage, input, c.Policy.Policy.IdentityFree)
	if !ok {
		return &Error{Usage, "record"}
	}
	record["fetched_at"] = c.now().Format(time.RFC3339Nano)
	record["privacy_version"] = 1
	encoded, err := json.Marshal(record)
	if err != nil {
		return &Error{Usage, "record"}
	}
	unlock, err := lockOwned(path, true)
	if err != nil {
		return &Error{Usage, "lock"}
	}
	defer unlock()
	if _, err := c.pruneTemporaryFiles(path, Usage); err != nil {
		return err
	}
	// Inspect the previous leaf without reading its contents; a symlink must not
	// authorize even an atomic replacement of an unowned/redirected cache entry.
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return &Error{Usage, "inspection"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return &Error{Usage, "inspection"}
	}
	if err := config.AtomicWriteFile(path, append(encoded, '\n')); err != nil {
		return &Error{Usage, "write"}
	}
	return nil
}

// Append minimizes the new record and prunes existing records under one OS lock.
// The application supplies the recording time; input cannot backdate/extend it.
func (c Controller) Append(path string, category Category, data []byte) error {
	if !c.Enabled() || !validCategory(category) || c.age(category) < 0 {
		return &Error{category, "policy"}
	}
	if c.age(category) == 0 {
		_, err := c.Prune(path, category, true)
		return err
	}
	if len(data) > maxRecordBytes || category == Usage {
		return &Error{category, "record"}
	}
	var input map[string]any
	if json.Unmarshal(data, &input) != nil {
		return &Error{category, "record"}
	}
	record, ok := minimize(category, input, c.Policy.Policy.IdentityFree)
	if !ok {
		return &Error{category, "record"}
	}
	record["ts"] = c.now().Format(time.RFC3339Nano)
	record["privacy_version"] = 1
	line, err := json.Marshal(record)
	if err != nil {
		return &Error{category, "record"}
	}
	unlock, err := lockOwned(path, true)
	if err != nil {
		return &Error{category, "lock"}
	}
	defer unlock()
	if _, err := c.pruneTemporaryFiles(path, category); err != nil {
		return err
	}
	old, _, err := readOwned(path, maxStoreBytes)
	if err != nil {
		return &Error{category, "read"}
	}
	kept, _ := c.filter(category, old, false)
	if len(kept)+len(line)+1 > maxStoreBytes {
		return &Error{category, "size limit"}
	}
	next := append(kept, line...)
	next = append(next, '\n')
	if err := config.AtomicWriteFile(path, next); err != nil {
		return &Error{category, "write"}
	}
	return nil
}

// RemoveLegacy removes one explicitly named owned legacy file without decoding
// its raw contents. It never follows links, recursively deletes, or prints paths.
func (c Controller) RemoveLegacy(path string, category Category) (Report, error) {
	if !c.Enabled() {
		return Report{}, nil
	}
	if err := safeParents(path); err != nil {
		return Report{Failed: 1}, &Error{category, "legacy inspection"}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return Report{}, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Report{Failed: 1}, &Error{category, "legacy inspection"}
	}
	if err := os.Remove(path); err != nil {
		return Report{Failed: 1}, &Error{category, "legacy deletion"}
	}
	return Report{DeletedFiles: 1}, nil
}

// Refuse redirected data directories, including project-controlled .which-model
// links. macOS's fixed system aliases are the only exception; their exact target
// is checked. This does not attempt to enforce OS ownership or defeat a process
// already running with the same user's filesystem authority.
func safeParents(path string) error {
	dir := filepath.Dir(path)
	for {
		info, err := os.Lstat(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				allowed := false
				if runtime.GOOS == "darwin" && (dir == "/var" || dir == "/tmp" || dir == "/etc") {
					target, err := filepath.EvalSymlinks(dir)
					allowed = err == nil && target == "/private"+dir
				}
				if !allowed {
					return errors.New("redirected data directory")
				}
			} else if !info.IsDir() {
				return errors.New("invalid data directory")
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}
