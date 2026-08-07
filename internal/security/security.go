// Package security ports the Node prototype's security core: token validation,
// bounded file I/O, URL validation, and the canary harness.
package security

// MaxCredentialBytes bounds credential files (specs/global/CONTRACTS.md §7).
const MaxCredentialBytes = 1_048_576 // 1 MiB

// MaxResponseBytes bounds HTTP response bodies (specs/global/CONTRACTS.md §7).
const MaxResponseBytes = 262_144 // 256 KiB

// Error is a domain error carrying a stable Failure.Code string from
// specs/global/CONTRACTS.md §1.6 and a fixed, sanitized message.
type Error struct {
	Code    string // unsafe_credential | credential_file | endpoint_refused | untrusted_origin | response_too_large
	Message string // fixed constant; never interpolates input or underlying errors
}

// Error renders "<code>: <message>".
func (e *Error) Error() string { return e.Code + ": " + e.Message }
