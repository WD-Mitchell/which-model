package security

import (
	"net/url"
	"slices"
	"strings"
)

// ValidateExactHTTPS ports validateExactHttpsUrl (core.mjs:76-91): rawURL must
// parse, be https, carry no userinfo and no fragment, have a non-empty host,
// and its Go-serialized form ((*url.URL).String()) must be an exact member of
// allowed — no prefix/origin/substring matching. Returns the canonical
// serialized URL. Parse failure -> Error{Code:"endpoint_refused",
// Message:"The provider endpoint is not a valid URL."}; any other rejection
// -> Error{Code:"endpoint_refused", Message:"The provider endpoint was refused."}
func ValidateExactHTTPS(rawURL string, allowed []string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", &Error{Code: "endpoint_refused", Message: "The provider endpoint is not a valid URL."}
	}
	if u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Host == "" || !slices.Contains(allowed, u.String()) {
		return "", &Error{Code: "endpoint_refused", Message: "The provider endpoint was refused."}
	}
	return u.String(), nil
}

// ValidateTrustedBaseURL ports validateTrustedBaseUrl (core.mjs:93-116):
// base must be https with no userinfo/query/fragment; trustedOrigin must be
// https with no userinfo/query/fragment and a bare path ("" or "/"); base's
// origin (scheme://host incl. port) must equal trustedOrigin's origin exactly.
// Returns the fallback target URL:
// base origin + base path (trailing slash ensured) + "api/codex/usage".
// Any violation -> Error{Code:"untrusted_origin",
// Message:"The configured Codex fallback origin was not explicitly trusted."}
func ValidateTrustedBaseURL(rawURL string, trustedOrigin string) (string, error) {
	reject := func() (string, error) {
		return "", &Error{Code: "untrusted_origin", Message: "The configured Codex fallback origin was not explicitly trusted."}
	}

	base, err := url.Parse(rawURL)
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

	path := base.Path
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	target := base.Scheme + "://" + base.Host + path + "api/codex/usage"
	// The Node original's defensive "target origin changed" guard is
	// unreachable under Go string construction and is omitted (SPEC.md D9).
	return target, nil
}
