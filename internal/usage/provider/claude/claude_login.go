//go:build !nousage

package claude

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Public Claude Code OAuth client (not a secret). Endpoints match current
// Claude Code production config: authorize via claude.ai, token + paste
// redirect on platform.claude.com.
const (
	ClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	AuthorizeURL = "https://claude.ai/oauth/authorize"
	TokenURL     = "https://platform.claude.com/v1/oauth/token"
	RedirectURI  = "https://platform.claude.com/oauth/code/callback"
	OAuthScope   = "org:create_api_key user:profile user:inference user:sessions:claude_code"
)

// Tokens is the subset of a Claude OAuth response we persist.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// BrowserLogin is one PKCE session. The user pastes code#state from the page.
type BrowserLogin struct {
	AuthorizeURL string
	RedirectURI  string
	ClientID     string
	TokenURL     string
	Verifier     string
	State        string
	HTTP         *http.Client
	ValidateURL  func(string) error
	MaxBytes     int64
}

func defaultLoginClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func defaultValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	_, err = security.ValidateExactHTTPS(raw, []string{raw})
	return err
}

func randomURLToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// StartBrowserLogin creates a PKCE session and the URL to open. No network.
func StartBrowserLogin() (*BrowserLogin, error) {
	verifier, err := randomURLToken()
	if err != nil {
		return nil, usage.NewFailureError("network", "Claude sign-in could not start.")
	}
	state, err := randomURLToken()
	if err != nil {
		return nil, usage.NewFailureError("network", "Claude sign-in could not start.")
	}
	q := url.Values{}
	q.Set("code", "true")
	q.Set("client_id", ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", RedirectURI)
	q.Set("scope", OAuthScope)
	q.Set("code_challenge", s256Challenge(verifier))
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	return &BrowserLogin{
		AuthorizeURL: AuthorizeURL + "?" + q.Encode(),
		RedirectURI:  RedirectURI,
		ClientID:     ClientID,
		TokenURL:     TokenURL,
		Verifier:     verifier,
		State:        state,
		HTTP:         defaultLoginClient(),
		ValidateURL:  defaultValidateURL,
		MaxBytes:     security.MaxResponseBytes,
	}, nil
}

// Exchange swaps the pasted authentication code for tokens.
func (b *BrowserLogin) Exchange(ctx context.Context, pasted string) (Tokens, error) {
	if b == nil {
		return Tokens{}, usage.NewFailureError("validation_failed", "Claude sign-in is not in progress.")
	}
	code, state, err := parsePastedCode(pasted, b.State)
	if err != nil {
		return Tokens{}, err
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"state":         state,
		"code_verifier": b.Verifier,
		"redirect_uri":  b.RedirectURI,
		"client_id":     b.ClientID,
	})
	tokenURL := b.TokenURL
	if tokenURL == "" {
		tokenURL = TokenURL
	}
	if b.ValidateURL != nil {
		if err := b.ValidateURL(tokenURL); err != nil {
			return Tokens{}, usage.NewFailureError("endpoint_refused", "The Claude login URL is not allowed.")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return Tokens{}, usage.NewFailureError("network", "The Claude login request failed.")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := b.HTTP
	if client == nil {
		client = defaultLoginClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Tokens{}, ctx.Err()
		}
		return Tokens{}, usage.NewFailureError("network", "The Claude login request failed.")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return Tokens{}, usage.NewFailureError("redirect_refused", "The provider attempted an unsafe redirect.")
	}
	max := b.MaxBytes
	if max <= 0 {
		max = security.MaxResponseBytes
	}
	raw, err := security.ReadResponseBounded(resp, max)
	if err != nil {
		return Tokens{}, usage.NewFailureError("network", "The Claude login request failed.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Tokens{}, usage.NewFailureError("provider_status", fmt.Sprintf("Claude token exchange failed (HTTP %d).", resp.StatusCode))
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || security.ValidateOpaqueToken(tok.AccessToken) != nil {
		return Tokens{}, usage.NewFailureError("unsupported_response", "Claude returned an unsupported token response.")
	}
	return Tokens{AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken, ExpiresIn: tok.ExpiresIn}, nil
}

func parsePastedCode(pasted, expectedState string) (code, state string, err error) {
	pasted = strings.TrimSpace(pasted)
	pasted = strings.Trim(pasted, `"'`)
	if pasted == "" {
		return "", "", usage.NewFailureError("validation_failed", "Paste the code from the Claude login page.")
	}
	code, state, ok := strings.Cut(pasted, "#")
	if !ok {
		code, state = pasted, expectedState
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" {
		return "", "", usage.NewFailureError("validation_failed", "Paste the code from the Claude login page.")
	}
	if expectedState != "" && state != expectedState {
		return "", "", usage.NewFailureError("validation_failed", "The pasted Claude login code does not match this sign-in.")
	}
	return code, state, nil
}

// PersistLogin writes ~/.claude/.credentials.json in the shape Claude Code
// and usage.AuthFile expect.
func PersistLogin(tok Tokens) error {
	if security.ValidateOpaqueToken(tok.AccessToken) != nil {
		return usage.NewFailureError("unsafe_credential", "The Claude access token is missing or unsafe.")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return usage.NewFailureError("credential_file", "Claude credentials could not be saved.")
	}
	path := filepath.Join(home, ".claude", ".credentials.json")
	oauth := map[string]any{
		"accessToken":  tok.AccessToken,
		"refreshToken": tok.RefreshToken,
	}
	if tok.ExpiresIn > 0 {
		oauth["expiresAt"] = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UnixMilli()
	}
	data, err := json.Marshal(map[string]any{"claudeAiOauth": oauth})
	if err != nil {
		return usage.NewFailureError("credential_file", "Claude credentials could not be saved.")
	}
	if err := config.AtomicWriteFile(path, append(data, '\n')); err != nil {
		return usage.NewFailureError("credential_file", "Claude credentials could not be saved.")
	}
	return nil
}
