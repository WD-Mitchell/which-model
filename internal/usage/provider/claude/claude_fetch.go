//go:build !nousage

package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// broadPermissionsWarning is the verbatim stderr warning for credential files
// with any group/other permission bit set (claude.mjs:59; SPEC D13).
const broadPermissionsWarning = "Warning: Claude credential permissions are broader than 0600; review them before continuing."

// usageHeaders is the exact five-header CodexBar set (annex-a §3.2, SPEC D3).
var usageHeaders = map[string]string{
	"Accept":         "application/json",
	"Content-Type":   "application/json",
	"anthropic-beta": "oauth-2025-04-20",
	"User-Agent":     UserAgent,
}

// requestJSON is the port of requestJson (core.mjs:146-190): exact-URL
// allow-list, redirect hard-fail, bounded body, JSON object parse, context
// deadline and transport-failure mapping (SPEC §2.5, D2).
//
// Returns (status, nil, nil) for non-2xx responses (the caller maps the
// status via mapStatus) and (status, body, nil) for 2xx responses with an
// object body. All failures are *Error with a fixed sanitized message.
func requestJSON(ctx context.Context, client *http.Client, url string, allowed []string, headers map[string]string) (int, json.RawMessage, error) {
	endpoint, err := security.ValidateExactHTTPS(url, allowed)
	if err != nil {
		return 0, nil, &Error{Code: "endpoint_refused", Message: "The provider endpoint was refused."}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, &Error{Code: "network", Message: "The provider request failed."}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Redirects are NEVER followed: a client copy hard-fails on any 3xx.
	c2 := *client
	c2.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c2.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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

	// 2xx: the body must be a non-empty JSON object.
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return 0, nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return 0, nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	return resp.StatusCode, json.RawMessage(trimmed), nil
}

// mapStatus ports statusError (core.mjs:192-200): 401/403 → unauthorized,
// 429 → rate_limited, any other non-200 → provider_status; 200 → nil.
func mapStatus(provider string, status int) *Error {
	switch status {
	case 200:
		return nil
	case 401, 403:
		return &Error{Code: "unauthorized", Message: provider + " rejected the credential."}
	case 429:
		return &Error{Code: "rate_limited", Message: provider + " rate-limited the usage request."}
	default:
		return &Error{Code: "provider_status", Message: fmt.Sprintf("%s usage is unavailable (HTTP %d).", provider, status)}
	}
}

// Fetch is the FetchFunc port of checkClaudeUsage (claude.mjs:58-91) per
// SPEC §2 items 2-6, 10. It returns (Snapshot, nil) with Snapshot.Failure set
// for provider-level failures, or (Snapshot{}, error) for programming errors
// only; provider errors are *Error with a global Failure.Code.
func Fetch(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
	now := time.Now().UTC()
	failureSnapshot := func(f *usage.Failure) usage.Snapshot {
		return usage.Snapshot{
			Provider:   "claude",
			FetchedAt:  now,
			Source:     usage.SourceOAuth,
			Confidence: "live",
			Failure:    f,
		}
	}

	if cred.Token == "" {
		return failureSnapshot(&usage.Failure{
			Code:    "credential_file",
			Message: "Claude credentials were not found; sign in with Claude Code first.",
		}), nil
	}

	// File-sourced credentials are enriched, not re-resolved (SPEC D2): the
	// file leg re-reads the two declared paths (dot-file first) to enforce
	// the prototype's expiry check and broad-permission warning.
	token := cred.Token
	if cred.Source == usage.AuthFile {
		fc, err := LoadFileCredential(filepath.Join(homeDir(), DotFileRelativePath), filepath.Join(homeDir(), PlainFileRelativePath), time.Now())
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				return failureSnapshot(&usage.Failure{Code: e.Code, Message: e.Message}), nil
			}
			return usage.Snapshot{}, err
		}
		if fc.Token != "" {
			if fc.ExpiresAt != nil && !fc.ExpiresAt.After(time.Now()) {
				return failureSnapshot(&usage.Failure{
					Code:    "expired_credential",
					Message: "The Claude access token is expired.",
				}), nil
			}
			token = fc.Token
			if fc.BroadPermissions {
				log.Print(broadPermissionsWarning)
			}
		}
	}

	headers := make(map[string]string, len(usageHeaders)+1)
	for k, v := range usageHeaders {
		headers[k] = v
	}
	headers["Authorization"] = "Bearer " + token

	status, body, err := requestJSON(ctx, client, UsageURL, []string{UsageURL}, headers)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			return failureSnapshot(&usage.Failure{Code: e.Code, Message: e.Message}), nil
		}
		return usage.Snapshot{}, err
	}
	if status != 200 {
		e := mapStatus("Claude", status)
		return failureSnapshot(&usage.Failure{Code: e.Code, Message: e.Message}), nil
	}

	windows, err := NormalizeUsage(body)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			return failureSnapshot(&usage.Failure{Code: e.Code, Message: e.Message}), nil
		}
		return usage.Snapshot{}, err
	}
	return usage.Snapshot{
		Provider:   "claude",
		Windows:    windows,
		UsageKnown: slices.ContainsFunc(windows, func(w usage.Window) bool { return w.UsageKnown && !w.Synthetic }),
		FetchedAt:  now,
		Source:     usage.SourceOAuth,
		Confidence: "live",
	}, nil
}

// homeDir returns the user's home directory; on failure it returns "" so the
// file leg falls back to the chain credential (enrichment is best-effort).
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
