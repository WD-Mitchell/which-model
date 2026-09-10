package privacy

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/WD-Mitchell/which-model/internal/config"
)

type Layout struct {
	CacheDirs   []string
	StateDir    string
	ProjectRoot string
}

type Summary struct {
	Managed    bool                `json:"managed"`
	Categories map[Category]Report `json:"categories"`
}

func DefaultLayout() (Layout, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Layout{}, &Error{Usage, "location"}
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Layout{}, &Error{Usage, "location"}
	}
	paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)
	cwd, _ := os.Getwd()
	return Layout{CacheDirs: []string{filepath.Join(cache, "which-model", "usage-cache"), filepath.Join(paths.CacheDir, "usage-cache")}, StateDir: paths.StateDir, ProjectRoot: ProjectRoot(cwd)}, nil
}

// ProjectRoot selects only the current/explicit workspace. No home-directory or
// repository inventory is created or persisted for discovering past projects.
func ProjectRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	for {
		for _, name := range []string{".git", ".which-model"} {
			if _, err := os.Lstat(filepath.Join(cwd, name)); err == nil {
				return cwd
			}
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return ""
		}
		cwd = parent
	}
}

var cacheName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}\.json$`)

// A cache contains a small provider inventory. Bound directory enumeration even
// when the directory contains unrelated entries. Report incomplete work at the
// limit instead of claiming every entry was inspected.
func cacheEntries(dir string) ([]os.DirEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(1025)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if len(entries) > 1024 {
		return nil, errors.New("cache inventory limit")
	}
	return entries, err
}

func addReport(a, b Report) Report {
	return Report{Retained: a.Retained + b.Retained, Removed: a.Removed + b.Removed, Scrubbed: a.Scrubbed + b.Scrubbed, DeletedFiles: a.DeletedFiles + b.DeletedFiles, Failed: a.Failed + b.Failed}
}

// Maintain touches only the named product stores in the selected layout.
// Empty categories selects all categories. Filesystem errors retain explicit
// failure counts; successful deletion of one file does not hide another failure.
func (c Controller) Maintain(layout Layout, purge bool, categories []Category) (Summary, error) {
	result := Summary{Managed: c.Enabled(), Categories: map[Category]Report{}}
	if !c.Enabled() {
		return result, nil
	}
	wanted := map[Category]bool{}
	if len(categories) == 0 {
		categories = []Category{Usage, History, Audit, Launch}
	}
	for _, category := range categories {
		if c.age(category) <= 0 {
			return result, &Error{category, "selection"}
		}
		wanted[category] = true
		result.Categories[category] = Report{}
	}
	var failures []error
	apply := func(category Category, report Report, err error) {
		result.Categories[category] = addReport(result.Categories[category], report)
		if err != nil {
			failures = append(failures, err)
		}
	}
	if wanted[Usage] {
		seen := map[string]bool{}
		for _, dir := range layout.CacheDirs {
			if dir == "" || seen[filepath.Clean(dir)] {
				continue
			}
			seen[filepath.Clean(dir)] = true
			if err := safeParents(filepath.Join(dir, "entry")); err != nil {
				apply(Usage, Report{Failed: 1}, &Error{Usage, "directory inspection"})
				continue
			}
			entries, err := cacheEntries(dir)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				apply(Usage, Report{Failed: 1}, &Error{Usage, "directory read"})
				continue
			}
			for _, entry := range entries {
				if cacheName.MatchString(entry.Name()) {
					report, err := c.Prune(filepath.Join(dir, entry.Name()), Usage, purge)
					apply(Usage, report, err)
				}
			}
		}
	}
	if layout.StateDir != "" {
		files := []struct {
			category Category
			parts    []string
		}{{History, []string{"pick", "history.jsonl"}}, {Launch, []string{"launch.jsonl"}}, {Audit, []string{"audit", "evidence.jsonl"}}, {Audit, []string{"audit", "mismatches.jsonl"}}}
		for _, file := range files {
			if wanted[file.category] {
				path := filepath.Join(append([]string{layout.StateDir}, file.parts...)...)
				report, err := c.Prune(path, file.category, purge)
				apply(file.category, report, err)
			}
		}
		if wanted[Launch] {
			report, err := c.RemoveLegacy(filepath.Join(layout.StateDir, "launch.log"), Launch)
			apply(Launch, report, err)
		}
	}
	if wanted[Audit] && layout.ProjectRoot != "" {
		for _, name := range []string{"evidence.jsonl", "audit-mismatches.jsonl"} {
			report, err := c.RemoveLegacy(filepath.Join(layout.ProjectRoot, ".which-model", name), Audit)
			apply(Audit, report, err)
		}
	}
	return result, errors.Join(failures...)
}
