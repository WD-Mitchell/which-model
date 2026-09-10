//go:build !nousage

package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"net/http"
	"strings"
	"testing"
	"time"
)

func syntheticJWT(account string, expires int64) string {
	payload, _ := json.Marshal(map[string]any{"https://api.openai.com/auth": map[string]string{"chatgpt_account_id": account}, "exp": expires})
	return "synthetic." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestManagedCodexCredentialMetadata(t *testing.T) {
	tokens := Tokens{AccessToken: syntheticJWT(canaryAcct, time.Now().Add(time.Hour).Unix()), IDToken: syntheticJWT(canaryAcct, 0), RefreshToken: "REFRESH_CANARY"}
	cred, err := tokens.ManagedCredential()
	if err != nil {
		t.Fatal(err)
	}
	if cred.Extra["account_id"] != canaryAcct || cred.Extra["expires_at"] == "" {
		t.Fatal("required metadata missing")
	}
	encoded, _ := json.Marshal(cred.Extra)
	if strings.Contains(string(encoded), tokens.IDToken) || strings.Contains(string(encoded), tokens.RefreshToken) {
		t.Fatal("unneeded tokens retained")
	}
	if _, err := (Tokens{AccessToken: "opaque-token", IDToken: "invalid"}).ManagedCredential(); err == nil {
		t.Fatal("missing account metadata accepted")
	}
}

func TestManagedCodexFetchDoesNotReopenProviderFiles(t *testing.T) {
	// A missing provider directory would make the legacy loader fail. A secure
	// credential must reach only the official usage endpoint without reading it.
	t.Setenv("CODEX_HOME", t.TempDir())
	transport := &stubTransport{fn: canned(200, fixtureCase6)}
	cred := usage.Credential{Token: canaryToken, Source: usage.AuthOAuthDeviceFlow, Extra: map[string]string{"managed_store": "keychain", "account_id": canaryAcct, "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}
	snap, err := Fetch(context.Background(), cred, &http.Client{Transport: transport})
	if err != nil || snap.Failure != nil || len(transport.reqs) != 1 {
		t.Fatalf("secure fetch failed: %v failure=%v requests=%d", err, snap.Failure, len(transport.reqs))
	}
	wantHeaders(t, transport.reqs[0])
	cred.Extra["expires_at"] = "2000-01-01T00:00:00Z"
	snap, err = Fetch(context.Background(), cred, &http.Client{Transport: transport})
	if err != nil || snap.Failure == nil || snap.Failure.Code != "expired_credential" || len(transport.reqs) != 1 {
		t.Fatal("expired secure credential reached network")
	}
}
