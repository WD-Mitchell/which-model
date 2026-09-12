package company

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
)

var errProtection = errors.New("policy protection check failed")

func parentPaths(path string) []string {
	var paths []string
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		paths = append(paths, dir)
		if filepath.Dir(dir) == dir {
			break
		}
	}
	slices.Reverse(paths)
	return paths
}

// readProtected never follows a policy symlink/reparse point. Once a managed
// file exists, all ancestors and the opened file must satisfy OS protection.
func readProtected(path string) ([]byte, error) {
	parents := parentPaths(path)
	// Probe components without following redirects, including dangling redirects
	// that could otherwise make a required marker look absent.
	for _, parent := range parents {
		info, err := os.Lstat(parent)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || isRedirect(info) {
			return nil, errProtection
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || isRedirect(info) || info.Size() > MaxPolicyBytes {
		return nil, errProtection
	}
	for _, parent := range parents {
		f, err := os.Open(parent)
		if err != nil {
			return nil, err
		}
		checkErr := trustOpened(f, true)
		closeErr := f.Close()
		if checkErr != nil {
			return nil, checkErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, errProtection
	}
	if err := trustOpened(f, false); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxPolicyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPolicyBytes {
		return nil, errProtection
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if isRedirect(current) || !os.SameFile(opened, current) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return nil, errProtection
	}
	if err := trustOpened(f, false); err != nil {
		return nil, err
	}
	return data, nil
}
