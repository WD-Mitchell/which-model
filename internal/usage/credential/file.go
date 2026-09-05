//go:build !nousage

package credential

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// FileResolver reads ordered candidate paths (SPEC §3). Missing path →
// ErrNotFound (next path); unreadable/oversized → credential_file;
// invalid JSON / non-object → credential_json; missing JSONPath value →
// ErrNotFound; unsafe token → unsafe_credential; expired (ExpiryPath) →
// expired_credential. ExtraPaths populate Credential.Extra.
type FileResolver struct {
	Paths      []string
	JSONPath   string
	ExtraPaths map[string]string
	ExpiryPath string

	// lastWarnings holds the permission warnings recorded by the most
	// recent Resolve call (SPEC §4); only the winning file is reported.
	lastWarnings []string
}

// Resolve walks r.Paths in order and returns the first credential that
// parses and validates. The winning file's mode is carried on the
// Credential for permission-warning purposes (SPEC §3).
func (r *FileResolver) Resolve(ctx context.Context) (usage.Credential, error) {
	r.lastWarnings = nil
	for _, candidate := range r.Paths {
		path, usable := expandCredentialPath(candidate)
		if !usable {
			continue
		}
		// A nonexistent path is "no candidate from this source — try the
		// next path"; every other read/stat failure is a hard
		// credential_file error (SPEC §3).
		info, statErr := os.Stat(path)
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
			return Credential{}, usage.NewFailureError("credential_file", "The credential file could not be read safely.")
		}
		if !info.Mode().IsRegular() {
			continue
		}

		data, mode, err := security.ReadBoundedFile(path, security.MaxCredentialBytes)
		if err != nil {
			return Credential{}, mapSecurityError(err, "credential_file")
		}

		obj, err := parseCredentialObject(data)
		if err != nil {
			return Credential{}, usage.NewFailureError("credential_json", "The credential file is not valid JSON.")
		}

		token, ok := walkStringPath(obj, r.JSONPath)
		if !ok || token == "" {
			continue // SPEC D4: missing/empty value → candidate unavailable
		}
		if err := security.ValidateOpaqueToken(token); err != nil {
			return Credential{}, mapSecurityError(err, "unsafe_credential")
		}

		if r.ExpiryPath != "" {
			if err := r.checkExpiry(obj); err != nil {
				return Credential{}, err
			}
		}

		extra := r.extractExtras(obj)

		// SPEC §4: warn on broad permissions, never remediate.
		if security.HasBroadPermissions(mode) {
			r.lastWarnings = append(r.lastWarnings, fmt.Sprintf(
				"credential file %s has broad permissions (%s); review before continuing", path, mode))
		}

		return Credential{Token: token, Extra: extra, Source: usage.AuthFile, Mode: uint32(mode.Perm())}, nil
	}
	return Credential{}, ErrNotFound
}

// expandCredentialPath expands descriptor placeholders without a shell. Values
// inserted from the environment and home directory are literal, never expanded
// recursively. An absent variable invalidates the whole candidate.
func expandCredentialPath(path string) (string, bool) {
	homePrefix := strings.HasPrefix(path, "~/")
	usable := true
	path = os.Expand(path, func(name string) string {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			usable = false
		}
		return value
	})
	if !usable {
		return "", false
	}
	if homePrefix {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", false
		}
		path = home + path[1:]
	}
	return path, path != ""
}

// checkExpiry extracts the ExpiryPath value and fails closed
// (expired_credential) when it is missing, unparseable, or in the past
// (SPEC §3 fail-safe).
func (r *FileResolver) checkExpiry(obj map[string]json.RawMessage) error {
	rawV, ok := walkRawPath(obj, r.ExpiryPath)
	if !ok {
		return usage.NewFailureError("expired_credential", "credential expired")
	}
	var v any
	if err := json.Unmarshal(rawV, &v); err != nil {
		return usage.NewFailureError("expired_credential", "credential expired")
	}
	exp, err := ParseExpiry(v)
	if err != nil {
		return usage.NewFailureError("expired_credential", "credential expired")
	}
	return CheckExpired(exp, time.Now())
}

// extractExtras copies each declared ExtraPaths name → dotted path value
// that is present and string-typed; missing/non-string entries are omitted
// (SPEC §3).
func (r *FileResolver) extractExtras(obj map[string]json.RawMessage) map[string]string {
	if len(r.ExtraPaths) == 0 {
		return nil
	}
	extra := make(map[string]string, len(r.ExtraPaths))
	for name, dotted := range r.ExtraPaths {
		if v, ok := walkStringPath(obj, dotted); ok {
			extra[name] = v
		}
	}
	return extra
}

// Warnings returns the permission warnings recorded by the last Resolve
// call (SPEC §4). Empty when the winning file mode is 0600/0700-clean.
func (r *FileResolver) Warnings() []string {
	return r.lastWarnings
}

// mapSecurityError converts an internal/security error into the canonical
// *usage.FailureError with the same fixed, sanitised message. fallbackCode
// guards against non-security errors (which carry no code of their own).
func mapSecurityError(err error, fallbackCode string) error {
	var se *security.Error
	if errors.As(err, &se) {
		return usage.NewFailureError(se.Code, se.Message)
	}
	return usage.NewFailureError(fallbackCode, "The credential file could not be read safely.")
}

// parseCredentialObject requires the bytes to decode to a non-null,
// non-array JSON object (prototype readCredentialJson). A JSON array
// decodes into [] for map[string]json.RawMessage, so the first
// non-whitespace byte is checked to be '{' before unmarshalling.
func parseCredentialObject(data []byte) (map[string]json.RawMessage, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("credential file is not a JSON object")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// walkRawPath resolves a dotted path to its raw JSON value, traversing
// only plain object members (arrays or scalars on the path → not found).
func walkRawPath(obj map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	cur := obj
	for i, part := range parts {
		rawV, ok := cur[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return rawV, true
		}
		var next map[string]json.RawMessage
		if err := json.Unmarshal(rawV, &next); err != nil {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// walkStringPath resolves a dotted path ("tokens.access_token") through
// plain object members to a JSON string value. Only plain object/string
// members are traversed — arrays or scalars on the path yield not-found.
func walkStringPath(obj map[string]json.RawMessage, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	parts := strings.Split(path, ".")
	cur := obj
	for i, part := range parts {
		rawV, ok := cur[part]
		if !ok {
			return "", false
		}
		if i == len(parts)-1 {
			var s string
			if err := json.Unmarshal(rawV, &s); err != nil {
				return "", false
			}
			return s, true
		}
		var next map[string]json.RawMessage
		if err := json.Unmarshal(rawV, &next); err != nil {
			return "", false
		}
		cur = next
	}
	return "", false
}
