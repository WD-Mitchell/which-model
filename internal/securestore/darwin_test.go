//go:build darwin && !nousage

package securestore

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		code int32
		kind Kind
	}{{-25300, Missing}, {-25308, Locked}, {-25315, Locked}, {-25293, Denied}, {-128, Denied}, {-1, Unavailable}} {
		if got := classifyDarwin(tc.code); !errors.Is(got, &Error{tc.kind}) {
			t.Fatalf("OSStatus %d: %v", tc.code, got)
		}
	}
	if loadDarwin() == nil {
		t.Fatal("fixed native framework bindings unavailable")
	}
	if err := Native().Set("service", "account", strings.Repeat("x", 64*1024+1)); !errors.Is(err, &Error{TooLarge}) {
		t.Fatalf("oversized value: %v", err)
	}
}

// Fixture setup only. Production uses the native framework and never a helper.
func runSecurity(_ string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	data, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &Error{Unavailable}
	}
	return data, nil
}
