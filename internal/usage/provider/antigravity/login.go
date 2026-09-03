//go:build !nousage

// Package antigravity implements Google OAuth for the Antigravity usage
// provider exposed by CodexBar.
package antigravity

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	authorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenEndpoint         = "https://oauth2.googleapis.com/token"
	userInfoEndpoint      = "https://www.googleapis.com/oauth2/v2/userinfo"
	callbackPath          = "/callback"
	credentialPrefix      = "antigravity-oauth."

	// CredentialsEnvironment is read by CodexBar for an in-memory Antigravity
	// OAuth credential override.
	CredentialsEnvironment = "ANTIGRAVITY_OAUTH_CREDENTIALS_JSON"
)

var (
	clientIDPattern     = regexp.MustCompile(`[0-9]+-[A-Za-z0-9_-]+\.apps\.googleusercontent\.com`)
	clientSecretPattern = regexp.MustCompile(`GOCSPX-[A-Za-z0-9_-]{28}`)
)

type oauthClient struct {
	ID     string
	Secret string
}

type callback struct {
	code  string
	state string
	err   string
}

// Credentials is the JSON shape CodexBar accepts for Antigravity OAuth.
type Credentials struct {
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiryDate   float64 `json:"expiry_date"`
	IDToken      string  `json:"id_token,omitempty"`
	Email        string  `json:"email,omitempty"`
	ClientID     string  `json:"client_id"`
	ClientSecret string  `json:"client_secret"`
}

// BrowserLogin owns the temporary loopback callback server for one sign-in.
type BrowserLogin struct {
	VerificationURL string
	callbackURL     string
	state           string
	client          oauthClient
	callbacks       <-chan callback
	server          *http.Server
	httpClient      *http.Client
}

// StartBrowserLogin discovers Antigravity's installed desktop OAuth client and
// starts a loopback callback server before returning the Google authorization
// URL. The caller is responsible for opening VerificationURL.
func StartBrowserLogin(ctx context.Context, httpClient *http.Client) (*BrowserLogin, error) {
	client, err := discoverOAuthClient()
	if err != nil {
		return nil, errors.New("Antigravity OAuth is unavailable; install Antigravity or set its OAuth client environment variables")
	}
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, errors.New("could not start Antigravity sign-in")
	}
	state := hex.EncodeToString(stateBytes)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("could not reserve the Antigravity sign-in callback")
	}
	callbackURL := "http://" + listener.Addr().String() + callbackPath
	callbacks := make(chan callback, 1)
	mux := http.NewServeMux()
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    8 << 10,
		Handler:           mux,
	}
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		result := callback{
			code:  strings.TrimSpace(query.Get("code")),
			state: strings.TrimSpace(query.Get("state")),
			err:   strings.TrimSpace(query.Get("error")),
		}
		stateMatches := result.state == state
		valid := request.Method == http.MethodGet && result.err == "" && result.code != "" && stateMatches
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if !valid {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, callbackPage(false))
		} else {
			_, _ = io.WriteString(w, callbackPage(true))
		}
		if request.Method == http.MethodGet && stateMatches {
			select {
			case callbacks <- result:
			default:
			}
		}
	})
	go func() {
		_ = server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	verificationURL, err := authorizationURL(client.ID, callbackURL, state)
	if err != nil {
		_ = server.Close()
		return nil, errors.New("could not build the Antigravity sign-in URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &BrowserLogin{
		VerificationURL: verificationURL,
		callbackURL:     callbackURL,
		state:           state,
		client:          client,
		callbacks:       callbacks,
		server:          server,
		httpClient:      httpClient,
	}, nil
}

// Wait receives the OAuth callback, exchanges its one-time code, and returns
// credentials ready for secure persistence.
func (l *BrowserLogin) Wait(ctx context.Context) (Credentials, error) {
	if l == nil || l.server == nil {
		return Credentials{}, errors.New("Antigravity sign-in was not started")
	}
	defer l.server.Close()
	select {
	case result := <-l.callbacks:
		if result.err == "access_denied" {
			return Credentials{}, errors.New("Antigravity sign-in was cancelled")
		}
		if result.err != "" || result.code == "" || result.state != l.state {
			return Credentials{}, errors.New("Antigravity sign-in returned an invalid callback")
		}
		return l.exchange(ctx, result.code)
	case <-ctx.Done():
		return Credentials{}, ctx.Err()
	}
}

// EncodeCredential converts credentials to the whitespace-free representation
// accepted by credential.ManagedStore.
func EncodeCredential(credentials Credentials) (string, error) {
	data, err := json.Marshal(credentials)
	if err != nil {
		return "", errors.New("could not encode Antigravity credentials")
	}
	return credentialPrefix + base64.RawURLEncoding.EncodeToString(data), nil
}

// CredentialsJSON unwraps a managed Antigravity token for CodexBar's process
// environment. Invalid values are rejected without exposing their contents.
func CredentialsJSON(token string) (string, bool) {
	encoded, ok := strings.CutPrefix(token, credentialPrefix)
	if !ok {
		return "", false
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	var credentials Credentials
	if err := json.Unmarshal(data, &credentials); err != nil ||
		credentials.AccessToken == "" || credentials.ClientID == "" || credentials.ClientSecret == "" {
		return "", false
	}
	return string(data), true
}

func (l *BrowserLogin) exchange(ctx context.Context, code string) (Credentials, error) {
	exchangeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	form := url.Values{
		"code":          {code},
		"client_id":     {l.client.ID},
		"client_secret": {l.client.Secret},
		"redirect_uri":  {l.callbackURL},
		"grant_type":    {"authorization_code"},
	}
	request, err := http.NewRequestWithContext(exchangeCtx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Credentials{}, errors.New("could not exchange the Antigravity sign-in code")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := l.httpClient.Do(request)
	if err != nil {
		return Credentials{}, errors.New("could not exchange the Antigravity sign-in code")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Credentials{}, errors.New("Antigravity rejected the sign-in code")
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil ||
		payload.AccessToken == "" || payload.ExpiresIn <= 0 ||
		payload.ExpiresIn > int64((30*24*time.Hour)/time.Second) {
		return Credentials{}, errors.New("Antigravity returned an invalid token response")
	}
	credentials := Credentials{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiryDate:   float64(time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second).UnixMilli()),
		IDToken:      payload.IDToken,
		ClientID:     l.client.ID,
		ClientSecret: l.client.Secret,
	}
	credentials.Email = l.fetchEmail(ctx, payload.AccessToken)
	return credentials, nil
}

func (l *BrowserLogin) fetchEmail(ctx context.Context, accessToken string) string {
	emailCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(emailCtx, http.MethodGet, userInfoEndpoint, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := l.httpClient.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var payload struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload) != nil {
		return ""
	}
	return strings.TrimSpace(payload.Email)
}

func authorizationURL(clientID, redirectURL, state string) (string, error) {
	parsed, err := url.Parse(authorizationEndpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email")
	query.Set("access_type", "offline")
	query.Set("prompt", "select_account consent")
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func callbackPage(success bool) string {
	if success {
		return "<!doctype html><title>Sign-in complete</title><p>Antigravity sign-in completed. You can close this window and return to which-model.</p>"
	}
	return "<!doctype html><title>Sign-in failed</title><p>Antigravity sign-in failed. You can close this window and try again.</p>"
}

func discoverOAuthClient() (oauthClient, error) {
	if id, secret := strings.TrimSpace(os.Getenv("ANTIGRAVITY_OAUTH_CLIENT_ID")), strings.TrimSpace(os.Getenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET")); id != "" && secret != "" {
		return oauthClient{ID: id, Secret: secret}, nil
	}
	for _, path := range oauthArtifactPaths() {
		ids, secrets := scanOAuthArtifact(path)
		if client, ok := preferredClient(ids, secrets); ok {
			return client, nil
		}
	}
	return oauthClient{}, exec.ErrNotFound
}

func oauthArtifactPaths() []string {
	var paths []string
	for _, name := range []string{"agy", "antigravity"} {
		if path, err := exec.LookPath(name); err == nil {
			paths = append(paths, path)
		}
	}
	home, _ := os.UserHomeDir()
	for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
		for _, relative := range []string{
			"Antigravity.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_arm",
			"Antigravity.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos_x64",
			"Antigravity.app/Contents/Resources/app/extensions/antigravity/bin/language_server_macos",
			"Antigravity.app/Contents/Resources/app/out/main.js",
			"Antigravity.app/Contents/Resources/bin/language_server",
			"Antigravity.app/Contents/Resources/bin/language_server_macos",
		} {
			paths = append(paths, filepath.Join(root, relative))
		}
	}
	return unique(paths)
}

func scanOAuthArtifact(path string) ([]string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1<<20)
	buffer := make([]byte, (1<<20)+256)
	carry := 0
	var ids, secrets []string
	for {
		n, readErr := reader.Read(buffer[carry:])
		data := buffer[:carry+n]
		for _, match := range clientIDPattern.FindAll(data, -1) {
			ids = append(ids, string(match))
		}
		for _, match := range clientSecretPattern.FindAll(data, -1) {
			secrets = append(secrets, string(match))
		}
		if readErr != nil {
			if readErr != io.EOF {
				return nil, nil
			}
			break
		}
		carry = min(256, len(data))
		copy(buffer[:carry], data[len(data)-carry:])
	}
	return unique(ids), unique(secrets)
}

func preferredClient(ids, secrets []string) (oauthClient, bool) {
	if len(ids) == 0 || len(secrets) == 0 {
		return oauthClient{}, false
	}
	if len(secrets) == 1 && len(ids) > 1 {
		return oauthClient{ID: ids[len(ids)-1], Secret: secrets[0]}, true
	}
	secret := secrets[0]
	if len(secrets) == len(ids) && len(secrets) > 1 {
		secret = secrets[len(secrets)-1]
	}
	return oauthClient{ID: ids[0], Secret: secret}, true
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
