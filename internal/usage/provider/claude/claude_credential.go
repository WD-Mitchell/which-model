//go:build !nousage

package claude

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// FileCredential is the enriched file-leg credential the Fetch derives when
// the chain resolved via AuthFile (SPEC D2).
type FileCredential struct {
	Token            string
	ExpiresAt        *time.Time // nil when the file carries no expiry
	BroadPermissions bool
}

// LoadFileCredential re-reads dotPath then plainPath (bounded, via
// security.ReadBoundedFile with security.MaxCredentialBytes) and extracts the
// tolerant shape value.claudeAiOauth ?? value.oauth ?? value, token
// accessToken ?? access_token, expiry expiresAt ?? expires_at (claude.mjs:17-31).
//
// Returns (FileCredential{}, nil) when neither file exists or neither carries
// a token — the caller falls back to the chain credential. Returns an error
// only for hard failures: credential_file (exists but unreadable/oversized,
// message "Claude credentials were not found; sign in with Claude Code first."
// for missing), credential_json (unparseable/non-object), unsafe_credential
// (token fails security.ValidateOpaqueToken), expired_credential (expiry
// known and past, or unparseable).
func LoadFileCredential(dotPath, plainPath string, now time.Time) (FileCredential, error) {
	for _, path := range []string{dotPath, plainPath} {
		data, mode, err := security.ReadBoundedFile(path, security.MaxCredentialBytes)
		if err != nil {
			var se *security.Error
			if errors.As(err, &se) && se.Code == "credential_file" && se.Message == "The credential file was not found." {
				continue // ENOENT/not-found → try the next path
			}
			return FileCredential{}, &Error{Code: "credential_file", Message: "Claude credentials were not found; sign in with Claude Code first."}
		}
		fc, err := parseCredentialFile(data, mode, now)
		if err != nil {
			return FileCredential{}, err
		}
		if fc.Token != "" {
			return fc, nil
		}
	}
	return FileCredential{}, nil
}

// parseCredentialFile decodes one credential file per claude.mjs:17-31:
// oauth = value.claudeAiOauth ?? value.oauth ?? value; token =
// oauth.accessToken ?? oauth.access_token; expiry = oauth.expiresAt ??
// oauth.expires_at (number or Date.parse-able string; > 10_000_000_000 is
// milliseconds, else seconds; unparseable or <= now → expired).
func parseCredentialFile(data []byte, mode fs.FileMode, now time.Time) (FileCredential, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return FileCredential{}, &Error{Code: "credential_json", Message: "The credential file is not valid JSON."}
	}

	// oauth = value.claudeAiOauth ?? value.oauth ?? value. A present (even
	// non-object) leading key wins, mirroring JS nullish coalescing: it never
	// falls through to the next candidate.
	oauthRaw := json.RawMessage(data)
	if raw, ok := value["claudeAiOauth"]; ok {
		oauthRaw = raw
	} else if raw, ok := value["oauth"]; ok {
		oauthRaw = raw
	}
	var oauth map[string]json.RawMessage
	if err := json.Unmarshal(oauthRaw, &oauth); err != nil || oauth == nil {
		return FileCredential{}, &Error{Code: "unsafe_credential", Message: "The Claude access token is missing or unsafe."}
	}

	// token = oauth.accessToken ?? oauth.access_token
	token, ok := stringField(oauth, "accessToken")
	if !ok {
		token, ok = stringField(oauth, "access_token")
	}
	if !ok || security.ValidateOpaqueToken(token) != nil {
		return FileCredential{}, &Error{Code: "unsafe_credential", Message: "The Claude access token is missing or unsafe."}
	}

	// expiry = oauth.expiresAt ?? oauth.expires_at
	expRaw, ok := oauth["expiresAt"]
	if !ok {
		expRaw, ok = oauth["expires_at"]
	}
	fc := FileCredential{Token: token, BroadPermissions: security.HasBroadPermissions(mode)}
	if !ok {
		return fc, nil
	}
	exp, err := parseExpiry(expRaw, now)
	if err != nil {
		return FileCredential{}, err
	}
	fc.ExpiresAt = exp
	return fc, nil
}

// parseExpiry ports the .mjs expiry heuristic (claude.mjs:24-28): a number is
// milliseconds when > 10_000_000_000 else seconds; a string is parsed as
// ISO-8601; unparseable or <= now → expired_credential.
func parseExpiry(raw json.RawMessage, now time.Time) (*time.Time, error) {
	trimmed := strings.TrimSpace(string(raw))
	var ms int64
	switch {
	case strings.HasPrefix(trimmed, `"`):
		var s string
		if err := json.Unmarshal([]byte(trimmed), &s); err != nil {
			return nil, expiredErr()
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, expiredErr()
		}
		ms = t.UnixMilli()
	case trimmed == "null" || trimmed == "true" || trimmed == "false":
		return nil, expiredErr()
	default:
		var n float64
		if err := json.Unmarshal([]byte(trimmed), &n); err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, expiredErr()
		}
		if n > 10_000_000_000 {
			ms = int64(n)
		} else {
			ms = int64(n) * 1000
		}
	}
	if ms <= now.UnixMilli() {
		return nil, expiredErr()
	}
	t := time.UnixMilli(ms).UTC()
	return &t, nil
}

func expiredErr() error {
	return &Error{Code: "expired_credential", Message: "The Claude access token is expired."}
}

// stringField returns the value of key as a JSON string, ok=false when the
// key is absent or not a string.
func stringField(obj map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := obj[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}
