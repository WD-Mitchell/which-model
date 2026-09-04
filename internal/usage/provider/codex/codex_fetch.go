//go:build !nousage

package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// requestJSON is the provider-local port of core.mjs requestJson (same helper
// contract as F15-T4): validates the exact allow-list, refuses redirects,
// bounds the response body, and maps transport errors. Returns (status, body)
// for non-2xx responses (the caller maps the status) and the parsed raw
// object body for 2xx.
func requestJSON(ctx context.Context, client *http.Client, url string, allowed []string, headers map[string]string) (int, json.RawMessage, error) {
	if _, err := security.ValidateExactHTTPS(url, allowed); err != nil {
		return 0, nil, &Error{Code: "endpoint_refused", Message: "The provider endpoint was refused."}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, &Error{Code: "network", Message: "The provider request failed."}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c2 := *client
	c2.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c2.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, nil, &Error{Code: "timeout", Message: "The provider request timed out."}
		}
		return 0, nil, &Error{Code: "network", Message: "The provider request failed."}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return 0, nil, &Error{Code: "redirect_refused", Message: "The provider attempted an unsafe redirect."}
	}
	body, err := security.ReadResponseBounded(resp, security.MaxResponseBytes)
	if err != nil {
		var se *security.Error
		if errors.As(err, &se) {
			return 0, nil, &Error{Code: se.Code, Message: se.Message}
		}
		return 0, nil, &Error{Code: "network", Message: "The provider request failed."}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, nil, nil
	}
	if len(body) == 0 {
		return 0, nil, &Error{Code: "response_json", Message: "The provider returned an empty response."}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return 0, nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	return resp.StatusCode, body, nil
}

// mapStatus ports statusError (core.mjs:192-200) for a provider name.
func mapStatus(provider string, status int) *Error {
	if status == 401 || status == 403 {
		return &Error{Code: "unauthorized", Message: provider + " rejected the credential."}
	}
	if status == 429 {
		return &Error{Code: "rate_limited", Message: provider + " rate-limited the usage request."}
	}
	return &Error{Code: "provider_status", Message: fmt.Sprintf("%s usage is unavailable (HTTP %d).", provider, status)}
}

// trustedFallbackURL is the provider-local port of validateTrustedBaseUrl
// (core.mjs:108-132): both URLs must parse, be https:, carry no
// userinfo/query/fragment; the trusted origin's pathname must be "/"; the
// origins must match exactly. The target is
// <base-origin><base-pathname-with-trailing-slash>api/codex/usage. A base
// pathname containing ".." segments is refused — under Go's non-normalizing
// string construction such a target could resolve outside the trusted
// origin (F16-T6 case 11).
func trustedFallbackURL(configuredBase, trustedOrigin string) (string, error) {
	reject := func() (string, error) {
		return "", &Error{Code: "untrusted_origin", Message: "The configured Codex fallback origin was not explicitly trusted."}
	}
	base, err := url.Parse(configuredBase)
	if err != nil {
		return reject()
	}
	trusted, err := url.Parse(trustedOrigin)
	if err != nil {
		return reject()
	}
	if base.Scheme != "https" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return reject()
	}
	if trusted.Scheme != "https" || trusted.User != nil || trusted.RawQuery != "" || trusted.Fragment != "" {
		return reject()
	}
	if trusted.Path != "" && trusted.Path != "/" {
		return reject()
	}
	if base.Scheme+"://"+base.Host != trusted.Scheme+"://"+trusted.Host {
		return reject()
	}
	for _, seg := range strings.Split(base.Path, "/") {
		if seg == ".." {
			return "", &Error{Code: "endpoint_refused", Message: "The configured Codex fallback endpoint was refused."}
		}
	}
	path := base.Path
	if path == "" {
		path = "/"
	} else if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return base.Scheme + "://" + base.Host + path + "api/codex/usage", nil
}

// resolveCredentialPaths defaults to $CODEX_HOME/{auth.json,config.toml} when
// CODEX_HOME is set, else ~/.codex/{auth.json,config.toml} (SPEC D2).
func resolveCredentialPaths() (authPath, configPath string) {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return filepath.Join(home, "auth.json"), filepath.Join(home, "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".codex", "auth.json"), filepath.Join(home, ".codex", "config.toml")
}

// codexHeaders is the exact three-header set fetchCodex sends (codex.mjs:83-92,
// SPEC D3 — no User-Agent).
func codexHeaders(cred Credential) map[string]string {
	return map[string]string{
		"Accept":             "application/json",
		"Authorization":      "Bearer " + cred.Token,
		"ChatGPT-Account-Id": cred.AccountID,
	}
}

// failureSnapshot wraps a provider Error into a Snapshot failure
// (CONTRACTS §9.4: (Snapshot{Provider:"codex", Failure: ...}, nil)).
func failureSnapshot(e *Error, now time.Time) (usage.Snapshot, error) {
	return usage.Snapshot{
		Provider:   "codex",
		Failure:    &usage.Failure{Code: e.Code, Message: e.Message},
		FetchedAt:  now,
		Source:     usage.SourceOAuth,
		Confidence: "live",
	}, nil
}

// Fetch is the FetchFunc port of checkCodexUsage (codex.mjs:102-117) per SPEC
// §2.5-§2.9, §2.12. The operational credential always comes from the verbatim
// loader (D1); cred is advisory. Provider failures are returned as
// (Snapshot{Provider:"codex", Failure: ...}, nil); the error return is
// reserved for programming errors. The trusted origin is read from ctx
// (WithTrustedOrigin).
func Fetch(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
	now := time.Now().UTC()
	authPath, configPath := resolveCredentialPaths()
	credential, err := LoadCredential(authPath, configPath)
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) {
			return failureSnapshot(ce, now)
		}
		return failureSnapshot(&Error{Code: "credential_file", Message: "The credential file could not be read safely."}, now)
	}

	headers := codexHeaders(credential)
	status, body, err := requestJSON(ctx, client, UsageURL, []string{UsageURL}, headers)
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) {
			return failureSnapshot(ce, now)
		}
		return failureSnapshot(&Error{Code: "network", Message: "The provider request failed."}, now)
	}
	if status == 200 {
		return finishFetch(now, body)
	}
	if !FallbackStatuses[status] {
		return failureSnapshot(mapStatus("Codex", status), now)
	}
	if credential.ConfiguredBaseURL == "" {
		return failureSnapshot(&Error{Code: "fallback_unavailable", Message: "Codex did not advertise a configured fallback endpoint."}, now)
	}
	target, err := trustedFallbackURL(credential.ConfiguredBaseURL, TrustedOriginFrom(ctx))
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) {
			return failureSnapshot(ce, now)
		}
		return failureSnapshot(&Error{Code: "untrusted_origin", Message: "The configured Codex fallback origin was not explicitly trusted."}, now)
	}
	status, body, err = requestJSON(ctx, client, target, []string{target}, headers)
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) {
			return failureSnapshot(ce, now)
		}
		return failureSnapshot(&Error{Code: "network", Message: "The provider request failed."}, now)
	}
	if status != 200 {
		return failureSnapshot(mapStatus("Codex fallback", status), now)
	}
	return finishFetch(now, body)
}

// finishFetch normalizes a 200 body into the success Snapshot (Account is
// never set — SPEC D7).
func finishFetch(now time.Time, body json.RawMessage) (usage.Snapshot, error) {
	windows, err := NormalizeUsage(body)
	if err != nil {
		var ce *Error
		if errors.As(err, &ce) {
			return failureSnapshot(ce, now)
		}
		return failureSnapshot(&Error{Code: "unsupported_response", Message: "Codex returned an unsupported usage shape."}, now)
	}
	return usage.Snapshot{
		Provider:   "codex",
		Windows:    windows,
		UsageKnown: slices.ContainsFunc(windows, func(w usage.Window) bool { return w.UsageKnown && !w.Synthetic }),
		FetchedAt:  now,
		Source:     usage.SourceOAuth,
		Confidence: "live",
	}, nil
}
