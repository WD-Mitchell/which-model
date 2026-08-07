package security

import "unicode"

// ValidateOpaqueToken ports assertOpaque (core.mjs:16-25): accepts a non-empty
// token of at most 8192 bytes with no whitespace, no control characters, and
// no DEL. Any violation returns Error{Code:"unsafe_credential",
// Message:"The credential is missing or unsafe."} — the token itself is never
// echoed. Returns nil on success.
func ValidateOpaqueToken(token string) error {
	if len(token) < 1 || len(token) > 8192 {
		return &Error{Code: "unsafe_credential", Message: "The credential is missing or unsafe."}
	}
	for _, r := range token {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return &Error{Code: "unsafe_credential", Message: "The credential is missing or unsafe."}
		}
	}
	return nil
}
