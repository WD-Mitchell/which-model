//go:build !nousage

package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// Error is the provider failure type. Code is always a value from
// specs/global/CONTRACTS.md §1.6; Message is a sanitized fixed string that
// never contains tokens, device codes, or logins (global SPEC §6 item 5).
type Error struct {
	Code    string
	Message string
}

// Error renders "<code>: <message>".
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// mapStatus is the status-to-Failure mapper (port of statusError,
// core.mjs:192-200; same shape as F15's): 401/403 → unauthorized, 429 →
// rate_limited, any other status → provider_status. Messages are the fixed
// CONTRACTS §6 strings with the provider name interpolated.
func mapStatus(provider string, status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Code: "unauthorized", Message: provider + " rejected the credential."}
	case http.StatusTooManyRequests:
		return &Error{Code: "rate_limited", Message: provider + " rate-limited the usage request."}
	default:
		return &Error{Code: "provider_status", Message: fmt.Sprintf("%s usage is unavailable (HTTP %d).", provider, status)}
	}
}

// requestJSON is the provider-agnostic GET helper implemented exactly as
// F15-T4 step 2 (same codes/messages): exact-URL allow-list
// (endpoint_refused), redirect hard-fail (redirect_refused), bounded body via
// F05 (response_too_large), transport errors (timeout when the context is
// done, else network), non-2xx → (status, nil, nil) for the caller to map;
// 2xx with an empty or non-object body → response_json.
func requestJSON(ctx context.Context, client *http.Client, url string, allowed []string, headers map[string]string) (int, json.RawMessage, error) {
	return doRequest(ctx, client, http.MethodGet, url, allowed, headers, nil)
}

// doRequest is the shared enforcement core behind requestJSON and the device
// flow's POSTs (SPEC §2.7: identical helper contract, provider-agnostic).
func doRequest(ctx context.Context, client *http.Client, method, url string, allowed []string, headers map[string]string, body io.Reader) (int, json.RawMessage, error) {
	// A context that is already done (deadline or cancellation) fails fast
	// before any request is issued — the port of the .mjs aborted-signal →
	// timeout mapping.
	if err := ctx.Err(); err != nil {
		return 0, nil, &Error{Code: "timeout", Message: "The provider request timed out."}
	}
	canonical, err := security.ValidateExactHTTPS(url, allowed)
	if err != nil {
		return 0, nil, &Error{Code: "endpoint_refused", Message: "The provider endpoint was refused."}
	}
	req, err := http.NewRequestWithContext(ctx, method, canonical, body)
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
		// A done context (deadline OR cancellation) maps to timeout; the
		// canonical deadline mapping from F15-T4 step 2 extends to an
		// already-cancelled context (F17-T5 case 10).
		if ctx.Err() != nil {
			return 0, nil, &Error{Code: "timeout", Message: "The provider request timed out."}
		}
		return 0, nil, &Error{Code: "network", Message: "The provider request failed."}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return 0, nil, &Error{Code: "redirect_refused", Message: "The provider attempted an unsafe redirect."}
	}
	raw, err := security.ReadResponseBounded(resp, security.MaxResponseBytes)
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
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return 0, nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	return resp.StatusCode, raw, nil
}

// loginPattern is the GitHub username charset/length rule
// (copilot.mjs:99-110; ^[A-Za-z0-9-]{1,39}$).
var loginPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)

// ValidateIdentity is the port of verifyGithubIdentity (copilot.mjs:99-110):
// GET GitHubUserURL with exactly the three headers {Accept:
// application/vnd.github+json, Authorization: Bearer <token>, User-Agent:
// IdentityUserAgent}; non-200 → Error per mapStatus("GitHub identity", status);
// login must match ^[A-Za-z0-9-]{1,39}$ else Error{Code: "unsupported_response",
// Message: "GitHub returned an unsupported identity response."}. Returns the
// login. This is the AuthSource.Validate hook for every chain entry of this
// provider (SPEC §2.1, D1).
func ValidateIdentity(ctx context.Context, token string, client *http.Client) (string, error) {
	status, raw, err := requestJSON(ctx, client, GitHubUserURL, []string{GitHubUserURL}, map[string]string{
		"Accept":        "application/vnd.github+json",
		"Authorization": "Bearer " + token,
		"User-Agent":    IdentityUserAgent,
	})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", mapStatus("GitHub identity", status)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	loginRaw, ok := value["login"]
	if !ok {
		return "", &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported identity response."}
	}
	var login string
	if err := json.Unmarshal(loginRaw, &login); err != nil {
		return "", &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported identity response."}
	}
	if !loginPattern.MatchString(login) {
		return "", &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported identity response."}
	}
	return login, nil
}
