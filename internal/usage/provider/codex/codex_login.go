//go:build !nousage

package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Public Codex CLI OAuth client (codex-rs/login). Not a secret.
const (
	Issuer   = "https://auth.openai.com"
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
)

const (
	defaultPollInterval = 5 * time.Second
	deviceAuthTimeout   = 15 * time.Minute
)

// Tokens is the subset of a Codex OAuth response we persist.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
}

// DeviceLogin is one OpenAI device-code session (not RFC 8628).
type DeviceLogin struct {
	Issuer          string
	ClientID        string
	DeviceAuthID    string
	UserCode        string
	VerificationURL string
	Interval        time.Duration
	HTTP            *http.Client
	Sleep           func(time.Duration)
	Now             func() time.Time
	ValidateURL     func(string) error
	MaxBytes        int64
	MaxWait         time.Duration
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
	// httptest servers are http on loopback; production issuers are https.
	host := u.Hostname()
	if u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	_, err = security.ValidateExactHTTPS(raw, []string{raw})
	return err
}

// StartDeviceLogin requests a user code from issuer (production: Issuer).
func StartDeviceLogin(ctx context.Context, issuer, clientID string, client *http.Client) (*DeviceLogin, error) {
	if client == nil {
		client = defaultLoginClient()
	}
	issuer = strings.TrimRight(issuer, "/")
	if clientID == "" {
		clientID = ClientID
	}
	login := &DeviceLogin{
		Issuer:      issuer,
		ClientID:    clientID,
		HTTP:        client,
		Sleep:       time.Sleep,
		Now:         time.Now,
		ValidateURL: defaultValidateURL,
		MaxBytes:    security.MaxResponseBytes,
		MaxWait:     deviceAuthTimeout,
	}
	usercodeURL := issuer + "/api/accounts/deviceauth/usercode"
	body, _ := json.Marshal(map[string]string{"client_id": clientID})
	status, raw, err := login.post(ctx, usercodeURL, "application/json", body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, usage.NewFailureError("provider_status", "Codex device login is not enabled for this account.")
	}
	if status < 200 || status >= 300 {
		return nil, usage.NewFailureError("provider_status", fmt.Sprintf("Codex device login is unavailable (HTTP %d).", status))
	}
	var resp struct {
		DeviceAuthID string          `json:"device_auth_id"`
		UserCode     string          `json:"user_code"`
		Usercode     string          `json:"usercode"`
		Interval     json.RawMessage `json:"interval"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || resp.DeviceAuthID == "" {
		return nil, usage.NewFailureError("unsupported_response", "Codex returned an unsupported device-login response.")
	}
	userCode := resp.UserCode
	if userCode == "" {
		userCode = resp.Usercode
	}
	if userCode == "" {
		return nil, usage.NewFailureError("unsupported_response", "Codex returned an unsupported device-login response.")
	}
	login.DeviceAuthID = resp.DeviceAuthID
	login.UserCode = userCode
	login.VerificationURL = issuer + "/codex/device"
	login.Interval = parseInterval(resp.Interval)
	if login.Interval <= 0 {
		login.Interval = defaultPollInterval
	}
	return login, nil
}

func parseInterval(raw json.RawMessage) time.Duration {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return time.Duration(n) * time.Second
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0
		}
		return time.Duration(v) * time.Second
	}
	return 0
}

// Wait polls until the user approves, then exchanges the authorization code.
func (d *DeviceLogin) Wait(ctx context.Context) (Tokens, error) {
	if d == nil {
		return Tokens{}, usage.NewFailureError("validation_failed", "Codex sign-in is not in progress.")
	}
	deadline := d.now().Add(d.maxWait())
	tokenURL := d.Issuer + "/api/accounts/deviceauth/token"
	payload, _ := json.Marshal(map[string]string{
		"device_auth_id": d.DeviceAuthID,
		"user_code":      d.UserCode,
	})
	var codes struct {
		AuthorizationCode string `json:"authorization_code"`
		CodeChallenge     string `json:"code_challenge"`
		CodeVerifier      string `json:"code_verifier"`
	}
	for {
		if ctx.Err() != nil {
			return Tokens{}, ctx.Err()
		}
		if !d.now().Before(deadline) {
			return Tokens{}, usage.NewFailureError("device_expired", "The Codex device code expired before it was approved.")
		}
		status, raw, err := d.post(ctx, tokenURL, "application/json", payload)
		if err != nil {
			return Tokens{}, err
		}
		if status == http.StatusOK {
			if err := json.Unmarshal(raw, &codes); err != nil || codes.AuthorizationCode == "" || codes.CodeVerifier == "" {
				return Tokens{}, usage.NewFailureError("unsupported_response", "Codex returned an unsupported device-login response.")
			}
			break
		}
		if status == http.StatusForbidden || status == http.StatusNotFound {
			sleepFor := d.Interval
			if remaining := deadline.Sub(d.now()); sleepFor > remaining {
				sleepFor = remaining
			}
			if sleepFor > 0 {
				d.sleep(sleepFor)
			}
			continue
		}
		return Tokens{}, usage.NewFailureError("provider_status", fmt.Sprintf("Codex device login failed (HTTP %d).", status))
	}
	return d.exchange(ctx, codes.AuthorizationCode, codes.CodeVerifier)
}

func (d *DeviceLogin) exchange(ctx context.Context, authorizationCode, codeVerifier string) (Tokens, error) {
	redirect := d.Issuer + "/deviceauth/callback"
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", authorizationCode)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", d.ClientID)
	form.Set("code_verifier", codeVerifier)
	status, raw, err := d.post(ctx, d.Issuer+"/oauth/token", "application/x-www-form-urlencoded", []byte(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	if status < 200 || status >= 300 {
		return Tokens{}, usage.NewFailureError("provider_status", fmt.Sprintf("Codex token exchange failed (HTTP %d).", status))
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || security.ValidateOpaqueToken(resp.AccessToken) != nil {
		return Tokens{}, usage.NewFailureError("unsupported_response", "Codex returned an unsupported token response.")
	}
	return Tokens{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken, IDToken: resp.IDToken}, nil
}

func (d *DeviceLogin) post(ctx context.Context, rawURL, contentType string, body []byte) (int, []byte, error) {
	if d.ValidateURL != nil {
		if err := d.ValidateURL(rawURL); err != nil {
			return 0, nil, usage.NewFailureError("endpoint_refused", "The Codex login URL is not allowed.")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, usage.NewFailureError("network", "The Codex login request failed.")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	resp, err := d.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, ctx.Err()
		}
		return 0, nil, usage.NewFailureError("network", "The Codex login request failed.")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return 0, nil, usage.NewFailureError("redirect_refused", "The provider attempted an unsafe redirect.")
	}
	max := d.MaxBytes
	if max <= 0 {
		max = security.MaxResponseBytes
	}
	raw, err := security.ReadResponseBounded(resp, max)
	if err != nil {
		return 0, nil, usage.NewFailureError("network", "The Codex login request failed.")
	}
	return resp.StatusCode, raw, nil
}

func (d *DeviceLogin) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *DeviceLogin) sleep(dur time.Duration) {
	if d.Sleep != nil {
		d.Sleep(dur)
		return
	}
	time.Sleep(dur)
}

func (d *DeviceLogin) maxWait() time.Duration {
	if d.MaxWait > 0 {
		return d.MaxWait
	}
	return deviceAuthTimeout
}

// PersistLogin writes ~/.codex/auth.json (or $CODEX_HOME/auth.json) in the
// shape Codex CLI and usage.AuthFile expect.
func PersistLogin(tok Tokens) error {
	if security.ValidateOpaqueToken(tok.AccessToken) != nil {
		return usage.NewFailureError("unsafe_credential", "The Codex access token is missing or unsafe.")
	}
	path, err := authFilePath()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"tokens": map[string]string{
			"access_token":  tok.AccessToken,
			"refresh_token": tok.RefreshToken,
			"id_token":      tok.IDToken,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return usage.NewFailureError("credential_file", "Codex credentials could not be saved.")
	}
	if err := config.AtomicWriteFile(path, append(data, '\n')); err != nil {
		return usage.NewFailureError("credential_file", "Codex credentials could not be saved.")
	}
	return nil
}

func authFilePath() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Join(home, "auth.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", usage.NewFailureError("credential_file", "Codex credentials could not be saved.")
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}
