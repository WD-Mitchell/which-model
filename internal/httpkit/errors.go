package httpkit

import (
	"errors"
	"net/url"
)

// Error is a domain error carrying a stable Failure.Code string from
// specs/global/CONTRACTS.md §1.6. StatusCode is the HTTP status (0 when the
// failure is not HTTP-level: network, timeout, parse, allow-list, redirect).
// Err holds the underlying cause for diagnosis; it is never rendered into
// Error(). Callers MUST branch on Code/StatusCode via errors.As — message
// text is sanitized and is NOT a contract (never match on it).
type Error struct {
	Code       string // endpoint_refused | redirect_refused | response_too_large | timeout | network | response_json | unauthorized | rate_limited | provider_status
	StatusCode int    // HTTP status; 0 when not an HTTP-level failure
	Err        error
}

// Error renders a fixed, sanitized message per code. It never includes
// request URLs, headers, tokens, or the underlying Err text.
func (e *Error) Error() string {
	return e.Code + ": " + fixedMessage(e)
}

// AsError extracts an *Error from err (or err itself), reporting whether
// the extraction succeeded.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func fixedMessage(e *Error) string {
	switch e.Code {
	case "endpoint_refused":
		var urlErr *url.Error
		if errors.As(e.Err, &urlErr) {
			return "the provider endpoint is not a valid URL"
		}
		return "the provider endpoint was refused"
	case "redirect_refused":
		return "the provider attempted an unsafe redirect"
	case "response_too_large":
		return "the provider response exceeded the safe size limit"
	case "timeout":
		return "the provider request timed out"
	case "network", "provider_status":
		return "the provider request failed"
	case "response_json":
		return "the provider returned unsupported JSON"
	case "unauthorized":
		return "the provider rejected the credential"
	case "rate_limited":
		return "the provider rate-limited the request"
	default:
		return "the request failed"
	}
}
