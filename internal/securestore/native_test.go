//go:build !nousage

package securestore

import (
	"errors"
	"os"
	"testing"
)

func nativeTestOnly(t *testing.T) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("WHICH_MODEL_NATIVE_STORE_TEST") != "1" {
		t.Skip("disposable native-store fixtures run only in explicit CI job")
	}
}

func roundTrip(t *testing.T, store Store, service string) {
	t.Helper()
	const account = "synthetic-account"
	const canary = "SYNTHETIC_CREDENTIAL_CANARY_283"
	t.Cleanup(func() { _ = store.Delete(service, account) })
	if _, err := store.Get(service, account); !errors.Is(err, &Error{Missing}) {
		t.Fatalf("missing item classification: %v", err)
	}
	for _, value := range []string{canary, canary + "_REPLACED"} {
		if err := store.Set(service, account, value); err != nil {
			t.Fatalf("native save: %v", err)
		}
		got, err := store.Get(service, account)
		if err != nil || got != value {
			t.Fatalf("native read-back mismatch (error: %v)", err)
		}
	}
	if err := store.Delete(service, account); err != nil {
		t.Fatalf("native delete: %v", err)
	}
	if _, err := store.Get(service, account); !errors.Is(err, &Error{Missing}) {
		t.Fatalf("deleted item: %v", err)
	}
}
