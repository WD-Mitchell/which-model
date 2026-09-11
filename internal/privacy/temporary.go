package privacy

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The supported Go writers use decimal uint32 suffixes for CreateTemp. Match
// only those reserved namespaces, not similarly named backups or lock files.
var temporarySuffix = regexp.MustCompile(`^[0-9]{1,10}$`)
var cacheTemporaryName = regexp.MustCompile(`^\.([a-z0-9][a-z0-9_-]{0,127}\.json)\.[0-9]{1,10}$`)
var legacyCacheTemporaryName = regexp.MustCompile(`^\.tmp-([a-z0-9][a-z0-9_-]{0,127})-[0-9]{1,10}$`)

func cacheRecordName(name string) string {
	if cacheName.MatchString(name) {
		return name
	}
	if match := cacheTemporaryName.FindStringSubmatch(name); match != nil {
		return match[1]
	}
	if match := legacyCacheTemporaryName.FindStringSubmatch(name); match != nil {
		return match[1] + ".json"
	}
	return ""
}

func temporaryFiles(path string, category Category) ([]string, error) {
	if err := safeParents(path); err != nil {
		return nil, err
	}
	entries, err := directoryEntries(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	base := filepath.Base(path)
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		suffix, current := strings.CutPrefix(name, "."+base+".")
		legacy := category == Usage && strings.HasPrefix(name, ".tmp-") && cacheRecordName(name) == base
		if (current && temporarySuffix.MatchString(suffix)) || legacy {
			files = append(files, filepath.Join(filepath.Dir(path), name))
		}
	}
	slices.Sort(files)
	return files, nil
}

// The caller must hold this store's lock. Re-enumerate after acquisition: an
// active writer may have renamed its temporary file while we waited. Remaining
// files were never committed and are discarded without decoding their payloads.
func (c Controller) pruneTemporaryFiles(path string, category Category) (Report, error) {
	files, err := temporaryFiles(path, category)
	if err != nil {
		return Report{Failed: 1}, &Error{category, "temporary inspection"}
	}
	var report Report
	var failures []error
	for _, file := range files {
		removed, err := c.RemoveLegacy(file, category)
		report = addReport(report, removed)
		if err != nil {
			failures = append(failures, &Error{category, "temporary deletion"})
		}
	}
	return report, errors.Join(failures...)
}
