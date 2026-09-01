//go:build !nousage

package whichmodel

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func stubAuthLogoutFileInfo(t *testing.T) {
	t.Helper()
	old := managedCredentialFileInfoFunc
	managedCredentialFileInfoFunc = func(string) (string, fs.FileMode, error) {
		return "", 0, nil
	}
	t.Cleanup(func() { managedCredentialFileInfoFunc = old })
}

func TestAuthLogoutConfirmed(t *testing.T) {
	stubAuthLogoutFileInfo(t)
	oldTTY, oldRemove := stdinIsTTY, removeFunc
	t.Cleanup(func() { stdinIsTTY, removeFunc = oldTTY, oldRemove })
	stdinIsTTY = func() bool { return true }
	removed := false
	removeFunc = func(string) error { removed = true; return nil }
	var out, errOut strings.Builder
	if err := RunAuthLogout("claude", false, &out, &errOut, strings.NewReader("y\n")); err != nil || !removed || out.String() != "Remove which-model's cached credential for claude? [y/N] " || strings.Contains(errOut.String(), "aborted") {
		t.Fatalf("err = %v, removed = %v, out = %q, errOut = %q", err, removed, out.String(), errOut.String())
	}
}

func TestAuthLogoutDeclined(t *testing.T) {
	oldTTY, oldRemove := stdinIsTTY, removeFunc
	t.Cleanup(func() { stdinIsTTY, removeFunc = oldTTY, oldRemove })
	stdinIsTTY = func() bool { return true }
	removed := false
	removeFunc = func(string) error { removed = true; return nil }
	var out, errOut strings.Builder
	if err := RunAuthLogout("claude", false, &out, &errOut, strings.NewReader("n\n")); err != nil || removed || !strings.Contains(errOut.String(), "aborted") {
		t.Fatalf("err = %v, removed = %v, out = %q, errOut = %q", err, removed, out.String(), errOut.String())
	}
}

func TestAuthLogoutRejectsUnattended(t *testing.T) {
	oldTTY, oldRemove := stdinIsTTY, removeFunc
	t.Cleanup(func() { stdinIsTTY, removeFunc = oldTTY, oldRemove })
	stdinIsTTY = func() bool { return false }
	called := false
	removeFunc = func(string) error { called = true; return nil }
	var out, errOut strings.Builder
	err := RunAuthLogout("claude", false, &out, &errOut, strings.NewReader("y\n"))
	if ExitCodeFor(err) != 2 || !strings.Contains(err.Error(), "refusing unattended logout without --yes") || called {
		t.Fatalf("err = %v, exit = %d, called = %v", err, ExitCodeFor(err), called)
	}
}

func TestAuthLogoutNothingToRemove(t *testing.T) {
	stubAuthLogoutFileInfo(t)
	oldTTY, oldRemove := stdinIsTTY, removeFunc
	t.Cleanup(func() { stdinIsTTY, removeFunc = oldTTY, oldRemove })
	stdinIsTTY = func() bool { return true }
	removeFunc = func(string) error { return errNoCredential }
	var out, errOut strings.Builder
	if err := RunAuthLogout("claude", true, &out, &errOut, strings.NewReader("")); err != nil || !strings.Contains(errOut.String(), "no which-model-managed credential for claude; nothing to remove") {
		t.Fatalf("err = %v, stderr = %q", err, errOut.String())
	}
}

func TestAuthLogoutRemovesUnresolvableManagedCredential(t *testing.T) {
	stubAuthLogoutFileInfo(t)
	oldTTY, oldResolve, oldRemove := stdinIsTTY, resolveFirstFunc, removeFunc
	t.Cleanup(func() { stdinIsTTY, resolveFirstFunc, removeFunc = oldTTY, oldResolve, oldRemove })
	stdinIsTTY = func() bool { return true }
	resolveFirstFunc = func(string) (AuthResolved, error) {
		t.Fatal("logout must not require credential resolution")
		return AuthResolved{}, errNoCredential
	}
	removed := false
	removeFunc = func(string) error { removed = true; return nil }
	var out, errOut strings.Builder
	if err := RunAuthLogout("claude", true, &out, &errOut, strings.NewReader("")); err != nil || !removed {
		t.Fatalf("err = %v, removed = %v, stderr = %q", err, removed, errOut.String())
	}
}

func TestAuthLogoutPermissionWarning(t *testing.T) {
	stubAuthLogoutFileInfo(t)
	oldTTY, oldRemove, oldPerms := stdinIsTTY, removeFunc, hasBroadPermsFunc
	t.Cleanup(func() {
		stdinIsTTY, removeFunc, hasBroadPermsFunc = oldTTY, oldRemove, oldPerms
	})
	stdinIsTTY = func() bool { return true }
	managedCredentialFileInfoFunc = func(string) (string, fs.FileMode, error) {
		return "/tmp/cred", fs.FileMode(0o644), nil
	}
	removeFunc = func(string) error { return nil }
	hasBroadPermsFunc = func(mode fs.FileMode) bool { return mode == 0o644 }
	var out, errOut strings.Builder
	if err := RunAuthLogout("claude", true, &out, &errOut, strings.NewReader("")); err != nil || strings.Count(errOut.String(), "Warning:") != 1 || !strings.Contains(errOut.String(), "Warning: /tmp/cred permissions are broader than 0600; review them.") {
		t.Fatalf("err = %v, stderr = %q", err, errOut.String())
	}
}

func TestAuthLogoutRemovalError(t *testing.T) {
	stubAuthLogoutFileInfo(t)
	oldTTY, oldRemove := stdinIsTTY, removeFunc
	t.Cleanup(func() { stdinIsTTY, removeFunc = oldTTY, oldRemove })
	stdinIsTTY = func() bool { return true }
	removeFunc = func(string) error { return errors.New("rm failed") }
	var out, errOut strings.Builder
	err := RunAuthLogout("claude", true, &out, &errOut, strings.NewReader(""))
	if ExitCodeFor(err) != 1 || !strings.Contains(errOut.String(), "[runtime] rm failed") {
		t.Fatalf("err = %v, exit = %d, stderr = %q", err, ExitCodeFor(err), errOut.String())
	}
}
