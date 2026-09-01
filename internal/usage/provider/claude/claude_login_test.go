//go:build !nousage

package claude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserLoginExchangeHappyPath(t *testing.T) {
	login, err := StartBrowserLogin()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(login.AuthorizeURL, "code=true") || !strings.Contains(login.AuthorizeURL, "code_challenge=") {
		t.Fatalf("authorize URL = %s", login.AuthorizeURL)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		if body["code"] != "pasted-code" || body["code_verifier"] != login.Verifier || body["state"] != login.State {
			t.Errorf("exchange body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"claude-access","refresh_token":"claude-refresh","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	login.TokenURL = srv.URL + "/v1/oauth/token"
	tok, err := login.Exchange(t.Context(), "pasted-code#"+login.State)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "claude-access" || tok.ExpiresIn != 3600 {
		t.Fatalf("tokens = %+v", tok)
	}
}

func TestParsePastedCodeRejectsStateMismatch(t *testing.T) {
	_, _, err := parsePastedCode("abc#other", "expected")
	if err == nil {
		t.Fatal("want state mismatch")
	}
}

func TestPersistLoginWritesCredentialsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := PersistLogin(Tokens{AccessToken: "claude-persist", RefreshToken: "refresh", ExpiresIn: 60}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	oauth, _ := parsed["claudeAiOauth"].(map[string]any)
	if oauth["accessToken"] != "claude-persist" {
		t.Fatalf("credentials = %s", data)
	}
}
