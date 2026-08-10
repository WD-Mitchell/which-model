//go:build !nousage

package credential

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// fakeKeychainStore is the only keychain the tests ever touch (SPEC D2).
type fakeKeychainStore struct {
	val string
	err error
}

func (f fakeKeychainStore) Get(service, account string) (string, error) {
	return f.val, f.err
}

func TestKeychainResolver(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	cases := []struct {
		name      string
		store     fakeKeychainStore
		wantTok   string
		wantErr   error
		wantCode  string // FailureError code when wantErr is nil but a hard error is expected
		wantStore string // Source when wantTok != ""
	}{
		{
			name:    "plain token", // case 1
			store:   fakeKeychainStore{val: "tok123"},
			wantTok: "tok123",
		},
		{
			name:    "not found", // case 2
			store:   fakeKeychainStore{err: keyringNotFound},
			wantErr: ErrNotFound,
		},
		{
			name:     "store error sanitised", // case 3
			store:    fakeKeychainStore{err: errors.New("bogus internal detail")},
			wantCode: "keychain_unavailable",
		},
		{
			name:    "empty value", // case 4
			store:   fakeKeychainStore{val: ""},
			wantErr: ErrNotFound,
		},
		{
			name:    "canary resolves", // case 5A
			store:   fakeKeychainStore{val: canary},
			wantTok: canary,
		},
		{
			name:     "canary in store error", // case 5B
			store:    fakeKeychainStore{err: errors.New("lookup failed: " + canary)},
			wantCode: "keychain_unavailable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &KeychainResolver{Store: tc.store, Service: "svc", Account: "acct"}
			cred, err := r.Resolve(context.Background())

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Resolve() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantCode != "" {
				if err == nil {
					t.Fatal("Resolve() = nil error, want keychain_unavailable")
				}
				f, ok := usage.AsFailure(err)
				if !ok || f.Code != tc.wantCode {
					t.Fatalf("Resolve() error = %v, want %s FailureError", err, tc.wantCode)
				}
				if strings.Contains(err.Error(), tc.store.err.Error()) {
					t.Fatalf("error %q leaks store error text %q", err, tc.store.err)
				}
				if strings.Contains(err.Error(), canary) {
					t.Fatalf("error %q leaks canary", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v, want nil", err)
			}
			if cred.Token != tc.wantTok {
				t.Fatalf("Resolve() token = %q, want %q", cred.Token, tc.wantTok)
			}
			if cred.Source != usage.AuthKeychainGeneric {
				t.Fatalf("Resolve() source = %v, want AuthKeychainGeneric", cred.Source)
			}
			if strings.Contains(cred.String(), canary) {
				t.Fatalf("Credential.String() = %q leaks canary", cred.String())
			}
		})
	}
}

func TestUnavailableKeychain(t *testing.T) { // case 6
	if _, err := (UnavailableKeychain{}).Get("s", "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UnavailableKeychain.Get() error = %v, want ErrNotFound", err)
	}
}

func TestDefaultKeychain(t *testing.T) { // case 7
	var _ KeychainStore = DefaultKeychain() // compile-time interface satisfaction
	if DefaultKeychain() == nil {
		t.Fatal("DefaultKeychain() = nil, want non-nil KeychainStore")
	}
}
