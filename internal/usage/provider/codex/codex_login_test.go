//go:build !nousage

package codex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeviceLoginHappyPath(t *testing.T) {
	var polls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/accounts/deviceauth/usercode":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"device_auth_id":"dev-1","user_code":"WDML-TEST","interval":"1"}`))
		case "/api/accounts/deviceauth/token":
			n := polls.Add(1)
			if n == 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"authorization_code":"auth-code","code_challenge":"ch","code_verifier":"ver"}`))
		case "/oauth/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"codex-access-token","refresh_token":"codex-refresh","id_token":"codex-id"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	login, err := StartDeviceLogin(t.Context(), srv.URL, "test-client", srv.Client())
	if err != nil {
		t.Fatalf("StartDeviceLogin: %v", err)
	}
	login.Sleep = func(time.Duration) {}
	login.Interval = time.Millisecond
	if login.UserCode != "WDML-TEST" || login.VerificationURL != srv.URL+"/codex/device" {
		t.Fatalf("login = %+v", login)
	}
	tok, err := login.Wait(t.Context())
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if tok.AccessToken != "codex-access-token" || tok.RefreshToken != "codex-refresh" {
		t.Fatalf("tokens = %+v", tok)
	}
}

func TestDeviceLoginUsercodeNotEnabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	_, err := StartDeviceLogin(t.Context(), srv.URL, "test-client", srv.Client())
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("err = %v, want not enabled", err)
	}
}

func TestPersistLoginWritesAuthJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	if err := PersistLogin(Tokens{AccessToken: "persist-access", RefreshToken: "persist-refresh"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	tokens, _ := parsed["tokens"].(map[string]any)
	if tokens["access_token"] != "persist-access" {
		t.Fatalf("auth.json = %s", data)
	}
}
