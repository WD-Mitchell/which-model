//go:build !nousage

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
)

type nativeFallbackTransport struct {
	calls   int
	request *http.Request
}

func (r *nativeFallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	r.request = req
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"rate_limit":{"primary_window":{"used_percent":20,"reset_at":1900000000}}}`)), Request: req}, nil
}

func TestSignInNativePersonalRollbackRemovesFallback(t *testing.T) {
	store := credential.ManagedStore{StateDir: t.TempDir(), NativeKeychain: true, Keychain: credential.UnavailableKeychain{}}
	if err := store.Save("codex", "SYNTHETIC_ACCESS"); err != nil {
		t.Fatal(err)
	}
	if err := restoreManagedCredential(store, "codex", usage.Credential{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path("codex")); !os.IsNotExist(err) {
		t.Fatalf("failed sign-in left a fallback credential: %v", err)
	}
}

func TestSignInNativeCodexPersonalFileFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	codexDir := t.TempDir()
	t.Setenv("CODEX_HOME", codexDir)
	stubModelsDevFetch(t, []modelsdev.ProviderModel{})
	payload, _ := json.Marshal(map[string]any{"https://api.openai.com/auth": map[string]string{"chatgpt_account_id": "synthetic-account"}, "exp": time.Now().Add(time.Hour).Unix()})
	token := "synthetic." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/deviceauth/usercode"):
			_, _ = w.Write([]byte(`{"device_auth_id":"dev-review","user_code":"REVIEW","interval":"1"}`))
		case strings.HasSuffix(r.URL.Path, "/deviceauth/token"):
			_, _ = w.Write([]byte(`{"authorization_code":"synthetic-code","code_challenge":"synthetic-challenge","code_verifier":"synthetic-verifier"}`))
		case strings.HasSuffix(r.URL.Path, "/oauth/token"):
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": token, "id_token": token})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	previousStart := startCodexLogin
	t.Cleanup(func() { startCodexLogin = previousStart; clearTestSignInFlow("codex") })
	startCodexLogin = func(ctx context.Context) (*codex.DeviceLogin, error) {
		login, err := codex.StartDeviceLogin(ctx, srv.URL, "synthetic-client", srv.Client())
		if err == nil {
			login.Sleep = func(time.Duration) {}
			login.Interval = time.Millisecond
		}
		return login, err
	}
	svc, _ := newTestServices(t, WithConfigTOML("[usage]\nbackend = \"native\"\n[auth]\nuse_keychain = false\nnative_keychain = true\n[providers.codex]\nenabled = true\n"))
	stubCatalogRepoFromCache(t, svc)
	started, err := svc.SignIn().Start(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SignIn().Confirm(context.Background(), "codex", started.FlowID, "Synthetic"); err != nil {
		t.Fatal(err)
	}
	store, err := svc.managedStoreFor("codex")
	if err != nil {
		t.Fatal(err)
	}
	cred, _, err := store.Resolve(context.Background(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Token != token || cred.Extra["account_id"] == "" {
		t.Fatal("login did not save expected synthetic credential/metadata")
	}
	if _, err := os.Stat(store.Path("codex")); err != nil {
		t.Fatal("expected successful file fallback")
	}
	data, err := os.ReadFile(store.Path("codex"))
	if err != nil || strings.Contains(string(data), "managed_store") {
		t.Fatal("resolution marker must not be persisted")
	}
	_, nativeFileErr := os.Stat(filepath.Join(codexDir, "auth.json"))
	if !os.IsNotExist(nativeFileErr) {
		t.Fatal("managed sign-in changed the provider credential file")
	}
	transport := &nativeFallbackTransport{}
	snap, err := codex.Fetch(context.Background(), cred, &http.Client{Transport: transport})
	if err != nil || snap.Failure != nil || transport.calls != 1 || cred.Extra["managed_store"] != "managed_file" {
		t.Fatalf("successful native-mode sign-in produced unusable file fallback: failure=%v err=%v HTTP requests=%d provider file absent=%t marker=%q", snap.Failure, err, transport.calls, os.IsNotExist(nativeFileErr), cred.Extra["managed_store"])
	}
	if transport.request.Header.Get("Authorization") != "Bearer "+token || transport.request.Header.Get("ChatGPT-Account-Id") != "synthetic-account" {
		t.Fatal("usage did not consume the saved token and account")
	}
}
