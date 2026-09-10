//go:build darwin && !nousage

package securestore

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDarwinStore(t *testing.T) {
	nativeTestOnly(t)
	path := filepath.Join(t.TempDir(), "disposable.keychain-db")
	// This synthetic password belongs only to a disposable CI keychain.
	if _, err := runSecurity("", "create-keychain", "-p", "which-model-ci-fixture", path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = runSecurity("", "delete-keychain", path) })
	if _, err := runSecurity("", "unlock-keychain", "-p", "which-model-ci-fixture", path); err != nil {
		t.Fatal(err)
	}
	store := darwinStore{keychain: path}
	roundTrip(t, store, "which-model-ci-native")
	if err := store.Set("which-model-ci-native", "locked", "SYNTHETIC_CANARY"); err != nil {
		t.Fatal(err)
	}
	if _, err := runSecurity("", "lock-keychain", path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("which-model-ci-native", "locked"); !errors.Is(err, &Error{Locked}) {
		t.Fatalf("locked read: %v", err)
	}
	if err := store.Set("which-model-ci-native", "locked", "SYNTHETIC_REPLACEMENT"); !errors.Is(err, &Error{Locked}) {
		t.Fatalf("locked write: %v", err)
	}
}

func TestDarwinErrorStates(t *testing.T) {
	for _, tc := range []struct {
		code string
		kind Kind
	}{{"44", Missing}, {"36", Locked}, {"29", Locked}, {"51", Denied}, {"128", Denied}, {"1", Unavailable}} {
		err := exec.Command("/bin/sh", "-c", "exit "+tc.code).Run()
		got := classifyDarwin(err)
		if !errors.Is(got, &Error{tc.kind}) {
			t.Fatalf("exit %s: %v", tc.code, got)
		}
	}
	if err := Native().Set("service", "account", strings.Repeat("x", 4096)); !errors.Is(err, &Error{TooLarge}) {
		t.Fatalf("oversized value: %v", err)
	}
}
