//go:build !darwin && !nousage

package credential

import (
	"context"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"net/http"
	"testing"
)

func TestDefaultKeychainFallsThroughToFile(t *testing.T) {
	path := writeCredFile(t, t.TempDir(), "auth.json", `{"token":"synthetic-file-token"}`, 0o600)
	cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{
		{Kind: usage.AuthKeychainGeneric, Keychain: &usage.KeychainSpec{Service: "synthetic-service"}},
		{Kind: usage.AuthFile, FilePaths: []string{path}, JSONPath: "token"},
	}, http.DefaultClient)
	if err != nil || cred.Source != usage.AuthFile {
		t.Fatalf("unavailable keychain must allow file fallback: %v", err)
	}
}
