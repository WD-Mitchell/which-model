//go:build !nousage

package antigravity

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestBrowserLoginExchangesCallbackAndEncodesCredential(t *testing.T) {
	const (
		clientID     = "123-test.apps." + "googleusercontent.com"
		clientSecret = "GOC" + "SPX-aaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	)
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_ID", clientID)
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET", clientSecret)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"access-test","refresh_token":"refresh-test","expires_in":3600,"id_token":"id-test"}`
		if request.URL.String() == userInfoEndpoint {
			body = `{"email":"user@example.com"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	login, err := StartBrowserLogin(ctx, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("StartBrowserLogin: %v", err)
	}
	verification, err := url.Parse(login.VerificationURL)
	if err != nil {
		t.Fatal(err)
	}
	query := verification.Query()
	if query.Get("client_secret") != "" || query.Get("client_id") != clientID || query.Get("state") == "" {
		t.Fatalf("authorization query is missing required public values or exposes the client secret")
	}
	callbackURL := query.Get("redirect_uri") + "?code=code-test&state=" + url.QueryEscape(query.Get("state"))
	response, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("OAuth callback: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("OAuth callback status = %d, want 200", response.StatusCode)
	}
	credentials, err := login.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if credentials.Email != "user@example.com" || credentials.AccessToken != "access-test" || credentials.ClientSecret != clientSecret {
		t.Fatalf("credentials did not contain the exchanged account data")
	}
	managed, err := EncodeCredential(credentials)
	if err != nil {
		t.Fatal(err)
	}
	credentialsJSON, ok := CredentialsJSON(managed)
	if !ok || !strings.Contains(credentialsJSON, `"access_token":"access-test"`) {
		t.Fatal("managed credential did not round-trip to CodexBar JSON")
	}
}

func TestBrowserLoginIgnoresStateMismatchBeforeValidCallback(t *testing.T) {
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_ID", "123-test.apps."+"googleusercontent.com")
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET", "GOC"+"SPX-aaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	calls := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"access-test","expires_in":3600}`
		if request.URL.String() == userInfoEndpoint {
			body = `{}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	login, err := StartBrowserLogin(ctx, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	verification, _ := url.Parse(login.VerificationURL)
	redirectURI := verification.Query().Get("redirect_uri")
	response, err := http.Get(redirectURI + "?code=code-test&state=wrong")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want 400", response.StatusCode)
	}
	if calls != 0 {
		t.Fatal("token endpoint called after state mismatch")
	}

	response, err = http.Get(redirectURI + "?code=code-test&state=" + url.QueryEscape(verification.Query().Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if _, err := login.Wait(context.Background()); err != nil {
		t.Fatalf("Wait after valid callback: %v", err)
	}
	if calls == 0 {
		t.Fatal("token endpoint was not called after valid callback")
	}
}

func TestExchangeRejectsUnboundedExpiry(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access-test","expires_in":9223372036854775807}`)),
			Request:    request,
		}, nil
	})
	login := &BrowserLogin{
		callbackURL: "http://127.0.0.1/callback",
		client:      oauthClient{ID: "client-id", Secret: "client-secret"},
		httpClient:  &http.Client{Transport: transport},
	}
	if _, err := login.exchange(context.Background(), "code-test"); err == nil {
		t.Fatal("exchange error = nil, want bounded expiry validation")
	}
}

func TestScanOAuthArtifactUsesAntigravityBinaryPairing(t *testing.T) {
	secretA := "GOCSPX-" + strings.Repeat("a", 28)
	secretB := "GOCSPX-" + strings.Repeat("b", 28)
	idA := "111-first.apps." + "googleusercontent.com"
	idB := "222-second.apps." + "googleusercontent.com"
	path := filepath.Join(t.TempDir(), "agy")
	if err := os.WriteFile(path, []byte(strings.Join([]string{secretA, secretB, idA, idB}, "\x00")), 0o600); err != nil {
		t.Fatal(err)
	}
	ids, secrets := scanOAuthArtifact(path)
	client, ok := preferredClient(ids, secrets)
	if !ok || client.ID != idA || client.Secret != secretB {
		t.Fatalf("preferredClient selected the wrong installed OAuth pair")
	}
}

func TestCredentialsJSONRejectsInvalidManagedValue(t *testing.T) {
	if _, ok := CredentialsJSON("not-a-managed-antigravity-value"); ok {
		t.Fatal("CredentialsJSON accepted an unrelated value")
	}
	credentials := Credentials{AccessToken: "token", ClientID: "id", ClientSecret: "secret", ExpiryDate: float64(time.Now().UnixMilli())}
	managed, err := EncodeCredential(credentials)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := CredentialsJSON(managed); !ok {
		t.Fatal("CredentialsJSON rejected a valid managed value")
	}
}
