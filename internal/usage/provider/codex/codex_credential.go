//go:build !nousage

package codex

import (
	"encoding/json"
	"errors"
	"unicode"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// Credential is the loader result (port of loadCodexCredential's return).
type Credential struct {
	Token             string
	AccountID         string
	ConfiguredBaseURL string // "" when neither auth.json nor config.toml configured one
}

// validateIdentifier ports assertIdentifier (core.mjs:24-31): a string of
// length 1..512 with no whitespace and no control characters.
func validateIdentifier(s string) bool {
	if len(s) < 1 || len(s) > 512 {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// asObjectField returns the JSON object stored under key; a missing key, JSON
// null, or any non-object value counts as absent (falls through, mirroring
// the .mjs `??` chain plus the SPEC's non-object-falls-through rule).
func asObjectField(m map[string]json.RawMessage, key string) (map[string]json.RawMessage, bool) {
	raw, ok := m[key]
	if !ok {
		return nil, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

// stringField decodes a JSON string field; non-strings decode to "".
func stringField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// LoadCredential is the port of loadCodexCredential (codex.mjs:52-61).
// auth.json is read bounded (security.ReadBoundedFile +
// security.MaxCredentialBytes). A missing/unreadable/oversized config.toml is
// silently ignored (returns "" for ConfiguredBaseURL).
func LoadCredential(authPath, configPath string) (Credential, error) {
	data, _, err := security.ReadBoundedFile(authPath, security.MaxCredentialBytes)
	if err != nil {
		var se *security.Error
		if errors.As(err, &se) {
			msg := se.Message
			if se.Message == "The credential file was not found." {
				msg = "Codex credentials were not found; sign in with Codex first."
			}
			return Credential{}, &Error{Code: "credential_file", Message: msg}
		}
		return Credential{}, &Error{Code: "credential_file", Message: "The credential file could not be read safely."}
	}

	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil || value == nil {
		return Credential{}, &Error{Code: "credential_json", Message: "The credential file is not valid JSON."}
	}

	tokens := value
	if obj, ok := asObjectField(value, "tokens"); ok {
		tokens = obj
	} else if obj, ok := asObjectField(value, "auth"); ok {
		tokens = obj
	}

	tokenRaw, ok := tokens["access_token"]
	if !ok {
		tokenRaw, ok = tokens["accessToken"]
	}
	token := stringField(tokenRaw)
	if !ok || security.ValidateOpaqueToken(token) != nil {
		return Credential{}, &Error{Code: "unsafe_credential", Message: "The Codex access token is missing or unsafe."}
	}

	acctRaw, ok := tokens["account_id"]
	if !ok {
		acctRaw, ok = tokens["accountId"]
	}
	if !ok {
		acctRaw, ok = value["account_id"]
	}
	if !ok {
		acctRaw, ok = value["chatgpt_account_id"]
	}
	accountID := stringField(acctRaw)
	if !ok || !validateIdentifier(accountID) {
		return Credential{}, &Error{Code: "unsafe_credential", Message: "The ChatGPT account identifier is missing or unsafe."}
	}

	configuredBaseURL := ""
	baseRaw, ok := value["base_url"]
	if !ok {
		baseRaw, ok = value["baseUrl"]
	}
	if !ok {
		baseRaw, ok = value["openai_base_url"]
	}
	if ok {
		configuredBaseURL = stringField(baseRaw)
	} else {
		// config.toml leg (optionalConfiguredBaseUrl, codex.mjs:37-41):
		// silently absent on any credential_file outcome.
		if data, _, err := security.ReadBoundedFile(configPath, security.MaxCredentialBytes); err == nil {
			configuredBaseURL = ParseConfig(string(data))
		}
	}

	return Credential{Token: token, AccountID: accountID, ConfiguredBaseURL: configuredBaseURL}, nil
}
