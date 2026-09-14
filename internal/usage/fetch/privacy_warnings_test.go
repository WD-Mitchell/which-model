//go:build !nousage

package fetch

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

func init() {
	for _, kind := range []string{"provider", "managed"} {
		id := "fake-warning-" + kind
		auth := usage.AuthSource{Kind: usage.AuthOAuthDeviceFlow}
		if kind == "provider" {
			auth = usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{"$WHICH_MODEL_TEST_WARNING_FILE"}, JSONPath: "token"}
		}
		usage.Register(usage.Descriptor{ID: id, Kind: usage.KindAPIKeyBilling, Auth: []usage.AuthSource{auth},
			Fetch: func(_ context.Context, cred usage.Credential, _ *http.Client) (usage.Snapshot, error) {
				if cred.Token != "synthetic-warning-token" {
					return usage.Snapshot{}, usage.NewFailureError("unauthorized", "fixture credential missing")
				}
				return usage.Snapshot{Provider: id}, nil
			},
		})
	}
}

func TestCompanyDiagnosticsCredentialWarnings(t *testing.T) {
	// Windows exposes regular file permission bits too; broad-permission files
	// must still produce the existing warning without leaking their path.
	for _, kind := range []string{"provider", "managed"} {
		for _, managed := range []bool{false, true} {
			name := kind + "/personal"
			if managed {
				name = kind + "/company"
			}
			t.Run(name, func(t *testing.T) {
				id := "fake-warning-" + kind
				old := readCompanyPolicy
				p := company.Defaults()
				p.AllowedProviders = []string{id}
				p.CredentialSources = []string{"provider_file", "managed_file"}
				p.SecureStoreOnly = false
				readCompanyPolicy = func() (company.Snapshot, error) {
					return company.Snapshot{Managed: managed, Policy: &p}, nil
				}
				t.Cleanup(func() { readCompanyPolicy = old })
				state := filepath.Join(t.TempDir(), "USER_IDENTITY_CANARY")
				path := filepath.Join(state, "auth.json")
				if kind == "managed" {
					path = (credential.ManagedStore{StateDir: state}).Path(id)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(`{"token":"synthetic-warning-token","source":"api_key"}`), 0644); err != nil {
					t.Fatal(err)
				}
				t.Setenv("WHICH_MODEL_TEST_WARNING_FILE", path)
				snaps, warns, err := FetchAll(context.Background(), []string{id}, Options{
					CacheDir: t.TempDir(), StateDir: state, DisableManagedKeychain: true,
					Enabled: map[string]bool{id: true},
				})
				if err != nil || len(snaps) != 1 || snaps[0].Failure != nil || len(warns) != 1 {
					t.Fatalf("%s: snapshots=%+v warnings=%+v err=%v", runtime.GOOS, snaps, warns, err)
				}
				if managed {
					if warns[0].Message != "credential file has broad permissions; review before continuing" {
						t.Fatalf("company warning exposes unexpected text: %q", warns[0].Message)
					}
				} else if !strings.Contains(warns[0].Message, "USER_IDENTITY_CANARY") {
					t.Fatal("personal permission warning lost its original path")
				}
			})
		}
	}
}

func TestCompanyDiagnosticsCacheWarning(t *testing.T) {
	old := readCompanyPolicy
	p := company.Defaults()
	p.AllowedProviders = []string{"fake-ok"}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	t.Cleanup(func() { readCompanyPolicy = old })
	dir := filepath.Join(t.TempDir(), "CACHE_IDENTITY_CANARY")
	if err := os.WriteFile(dir, []byte("block cache directory creation"), 0600); err != nil {
		t.Fatal(err)
	}
	snaps, warns, err := FetchAll(context.Background(), []string{"fake-ok"}, Options{
		Refresh: true, CacheDir: dir, Enabled: map[string]bool{"fake-ok": true},
	})
	if err != nil || len(snaps) != 1 || snaps[0].Failure != nil || len(warns) != 1 {
		t.Fatalf("cache write failure affected fetch: %+v %+v %v", snaps, warns, err)
	}
	if warns[0].Message != "usage cache write failed; run privacy cleanup for category results" {
		t.Fatalf("unexpected cache diagnostic: %q", warns[0].Message)
	}
}

func TestCompanyDiagnosticsUnknownWarnings(t *testing.T) {
	const fallback = "system keychain unavailable; using managed credential file"
	warnings := []credential.Warning{{Message: "UNKNOWN_PAYLOAD_CANARY"}, {Message: fallback}}
	minimizeCompanyWarnings(warnings)
	if strings.Contains(warnings[0].Message, "CANARY") || warnings[0].Message == "" || warnings[1].Message != fallback {
		t.Fatalf("unrecognized warning was exposed or fixed guidance was lost: %+v", warnings)
	}
}
