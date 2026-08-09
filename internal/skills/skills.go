// Package skills owns the agent-skill artifacts: the repo skills/ tree is the
// single source of truth, and Install/Remove/List place those files into
// harness-visible target directories (F28 agent skills). It imports only the
// Go standard library (specs/global/CONTRACTS.md §8).
package skills

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Target string

const (
	TargetGeneric Target = "generic"
	TargetClaude  Target = "claude"
)

// Names is the fixed skill set, in install order.
var Names = []string{"model-selection", "provider-usage", "usage-aware-dispatch"}

// RepoRoot walks upward from cwd to the nearest ancestor containing ".git".
// repoDir (the --repo flag) wins when non-empty. Error when neither exists.
func RepoRoot() (string, error) {
	if repoDir != "" {
		return repoDir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no repository root found (no .git ancestor); pass --repo <path>")
		}
		dir = parent
	}
}

// repoDir is set by the CLI layer from --repo (exported for tests).
var repoDir string

func SetRepoDir(path string) { repoDir = path }

// InstallDir returns the destination directory for name under target.
// user is only valid with TargetClaude (SPEC behaviour §2).
func InstallDir(root string, name string, target Target, user bool) (string, error) {
	switch target {
	case TargetGeneric:
		if user {
			return "", errors.New("--user is only supported with --target claude")
		}
		return filepath.Join(root, ".agents", "skills", name), nil
	case TargetClaude:
		if user {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(home, ".claude", "skills", name), nil
		}
		return filepath.Join(root, ".claude", "skills", name), nil
	default:
		return "", errors.New("unknown target: " + string(target))
	}
}

// installFile copies src to dst, writing only when dst is absent or
// byte-identical to src; a differing dst is refused unless force is set.
func installFile(src, dst string, force bool) error {
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if dstBytes, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(dstBytes, srcBytes) {
			return nil // already current: idempotent no-op
		}
		if !force {
			return fmt.Errorf("refusing to overwrite modified file %s (use --force)", dst)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, srcBytes, 0o644)
}

// removeFile deletes dst unless it differs from the shipped source (the
// repo skills/<name>/ counterpart); a differing file is refused unless
// force is set. A missing dst is a no-op.
func removeFile(src, dst string, force bool) error {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return nil
	}
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	dstBytes, err := os.ReadFile(dst)
	if err != nil {
		return err
	}
	if !bytes.Equal(srcBytes, dstBytes) && !force {
		return fmt.Errorf("refusing to delete modified file %s (use --force)", dst)
	}
	return os.Remove(dst)
}

// Install copies skills/<name>/SKILL.md and skills/<name>/agents/openai.yaml
// from the repo tree into the target dir. Returns a human message.
func Install(name string, target Target, user, force bool) (string, error) {
	if !validName(name) {
		return "", errors.New("unknown skill: " + name + " (known: " + strings.Join(Names, ", ") + ")")
	}
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	dir, err := InstallDir(root, name, target, user)
	if err != nil {
		return "", err
	}
	for _, rel := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml")} {
		if err := installFile(filepath.Join(root, "skills", name, rel), filepath.Join(dir, rel), force); err != nil {
			return "", err
		}
	}
	return "installed " + name + " to " + dir, nil
}

// Remove deletes the two installed files for name. Not-installed is a
// no-op success. Modified files are refused without force.
func Remove(name string, target Target, user, force bool) (string, error) {
	if !validName(name) {
		return "", errors.New("unknown skill: " + name + " (known: " + strings.Join(Names, ", ") + ")")
	}
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	dir, err := InstallDir(root, name, target, user)
	if err != nil {
		return "", err
	}
	removed := false
	for _, rel := range []string{"SKILL.md", filepath.Join("agents", "openai.yaml")} {
		dst := filepath.Join(dir, rel)
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			continue
		}
		if err := removeFile(filepath.Join(root, "skills", name, rel), dst, force); err != nil {
			return "", err
		}
		removed = true
	}
	if !removed {
		return name + " not installed (nothing to remove)", nil
	}
	// Best-effort cleanup of now-empty dirs.
	os.Remove(filepath.Join(dir, "agents"))
	os.Remove(dir)
	return "removed " + name + " from " + dir, nil
}

// List returns the installed skill names for the target.
func List(target Target, user bool) ([]string, error) {
	root, err := RepoRoot()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range Names {
		dir, err := InstallDir(root, name, target, user)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			out = append(out, name)
		}
	}
	return out, nil
}

func validName(name string) bool {
	for _, n := range Names {
		if n == name {
			return true
		}
	}
	return false
}
