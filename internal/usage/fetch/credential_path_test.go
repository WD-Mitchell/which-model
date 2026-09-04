//go:build !nousage

package fetch

import (
	"context"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeDescriptorExpandsFileCredential(t *testing.T) {
	for _, overridePresent := range []bool{true, false} {
		t.Run(fmtBool(overridePresent), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"token":"synthetic-descriptor-token"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("WHICH_MODEL_TEST_DESCRIPTOR_DIR", "")
			if overridePresent {
				t.Setenv("WHICH_MODEL_TEST_DESCRIPTOR_DIR", home)
			}
			calls := 0
			desc := usage.Descriptor{Kind: usage.KindSubscription, Auth: []usage.AuthSource{{Kind: usage.AuthFile, FilePaths: []string{"$WHICH_MODEL_TEST_DESCRIPTOR_DIR/auth.json", "~/auth.json"}, JSONPath: "token"}}, Fetch: func(_ context.Context, cred usage.Credential, _ *http.Client) (usage.Snapshot, error) {
				calls++
				if cred.Token != "synthetic-descriptor-token" {
					t.Error("wrong file credential")
				}
				return usage.Snapshot{Provider: "file-test"}, nil
			}}
			snap, _ := runProvider(context.Background(), &cache.Store{Dir: t.TempDir()}, http.DefaultClient, "file-test", desc, Options{StateDir: t.TempDir(), DisableManagedKeychain: true})
			if snap.Failure != nil || calls != 1 || snap.Source != usage.SourceAPI {
				t.Fatalf("expanded descriptor did not reach Fetch: calls=%d failure=%v", calls, snap.Failure)
			}
		})
	}
}

func fmtBool(value bool) string {
	if value {
		return "override present"
	}
	return "home fallback"
}
