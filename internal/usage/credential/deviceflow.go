//go:build !nousage

package credential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// DeviceFlow is the generic device-code state machine (SPEC §9).
// Now and Sleep are injectable test seams; defaults time.Now/time.Sleep.
// ValidateURL defaults to security.ValidateExactHTTPS(rawURL, []string{rawURL});
// tests replace it with a no-op to use httptest servers.
type DeviceFlow struct {
	Spec             usage.OAuthSpec
	HTTPClient       *http.Client // default: redirects hard-fail (CheckRedirect → http.ErrUseLastResponse)
	MaxResponseBytes int64        // <= 0 → security.MaxResponseBytes
	Now              func() time.Time
	Sleep            func(d time.Duration)
	ValidateURL      func(rawURL string) error
}

// NewDeviceFlow sets the transport defaults (SPEC D7, D9): a redirect-
// refusing client, the F05 response cap, wall-clock seams, and an exact
// HTTPS self-allow-list for the configured endpoints. VerificationURI is
// REQUIRED — a provider without one cannot run the flow safely (D9).
func NewDeviceFlow(spec usage.OAuthSpec) *DeviceFlow {
	if spec.VerificationURI == "" {
		panic("oauth: VerificationURI is required")
	}
	return &DeviceFlow{
		Spec: spec,
		HTTPClient: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		MaxResponseBytes: security.MaxResponseBytes,
		Now:              time.Now,
		Sleep:            time.Sleep,
		ValidateURL: func(raw string) error {
			_, err := security.ValidateExactHTTPS(raw, []string{raw})
			return err
		},
	}
}

// DeviceCode carries the validated device-flow state. Only UserCode and
// VerificationURI may ever be displayed; DeviceCode is opaque.
type DeviceCode struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

// userCodePattern is the GitHub-style display-code charset (SPEC §9).
var userCodePattern = regexp.MustCompile(`^[A-Z0-9-]{4,32}$`)

// Poll loops until the user approves, the code expires, or the context is
// cancelled (SPEC §9). Between polls it sleeps `code.Interval`; a
// slow_down response bumps the interval by 5s per RFC 8628 §3.5.
func (f *DeviceFlow) Poll(ctx context.Context, code DeviceCode) (string, error) {
	deadline := f.Now().Add(code.ExpiresIn)
	for {
		// Fail closed: an expired code or cancelled context never issues
		// another request (SPEC §9).
		if !f.Now().Before(deadline) || ctx.Err() != nil {
			return "", usage.NewFailureError("device_expired", "The device code expired before the user approved.")
		}
		token, err := f.pollOnce(ctx, code)
		switch {
		case err == nil:
			return token, nil
		case errors.Is(err, errDevicePending):
			f.Sleep(code.Interval)
		case errors.Is(err, errDeviceSlowDown):
			code.Interval += 5 * time.Second
			f.Sleep(code.Interval)
		default:
			return "", err
		}
	}
}

// errDevicePending / errDeviceSlowDown are loop-control signals returned by
// pollOnce; they never escape Poll.
var (
	errDevicePending  = errors.New("device flow: authorization pending")
	errDeviceSlowDown = errors.New("device flow: slow down requested")
)

// pollOnce issues a single token request. The client_secret is sent only
// when the spec supplies one (public clients omit it, RFC 8628 §3.2).
func (f *DeviceFlow) pollOnce(ctx context.Context, code DeviceCode) (string, error) {
	if err := f.ValidateURL(f.Spec.TokenURL); err != nil {
		return "", mapSecurityError(err, "endpoint_refused")
	}
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {code.DeviceCode},
		"client_id":   {f.Spec.ClientID},
	}
	if f.Spec.ClientSecret != "" {
		form.Set("client_secret", f.Spec.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.Spec.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", usage.NewFailureError("network", "The provider request failed.")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return "", usage.NewFailureError("network", "The provider request failed.")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// Redirects are never followed; rejected explicitly (SPEC §10).
		return "", usage.NewFailureError("redirect_refused", "The provider attempted an unsafe redirect.")
	}
	body, err := security.ReadResponseBounded(resp, f.effectiveMaxBytes())
	if err != nil {
		return "", usage.NewFailureError("network", "The provider request failed.")
	}
	if resp.StatusCode == http.StatusBadRequest {
		switch parseDeviceError(body) {
		case "authorization_pending":
			return "", errDevicePending
		case "slow_down":
			return "", errDeviceSlowDown
		case "access_denied":
			return "", usage.NewFailureError("access_denied", "The user denied the device login request.")
		case "expired_token":
			return "", usage.NewFailureError("device_expired", "The device code expired before the user approved.")
		case "":
			return "", usage.NewFailureError("provider_status",
				fmt.Sprintf("The device login endpoint is unavailable (HTTP %d).", resp.StatusCode))
		default:
			// Unknown RFC 8628 error strings are never passed through.
			return "", unsupportedResponse()
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", usage.NewFailureError("provider_status",
			fmt.Sprintf("The device login endpoint is unavailable (HTTP %d).", resp.StatusCode))
	}

	obj, err := parseDeviceObject(body)
	if err != nil {
		return "", unsupportedResponse()
	}
	var token string
	if !jsonString(obj["access_token"], &token) {
		return "", unsupportedResponse()
	}
	if err := security.ValidateOpaqueToken(token); err != nil {
		return "", unsupportedResponse()
	}
	return token, nil
}

// parseDeviceError extracts the RFC 8628 `error` field from a 400 body.
func parseDeviceError(data []byte) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	var e string
	if raw, ok := obj["error"]; ok {
		_ = json.Unmarshal(raw, &e)
	}
	return e
}

// Start POSTs DeviceCodeURL (form client_id+scope) and validates every
// response field (SPEC §9). Violations → unsupported_response.
func (f *DeviceFlow) Start(ctx context.Context) (DeviceCode, error) {
	if err := f.ValidateURL(f.Spec.DeviceCodeURL); err != nil {
		return DeviceCode{}, mapSecurityError(err, "endpoint_refused")
	}
	endpoint := f.Spec.DeviceCodeURL
	form := url.Values{"client_id": {f.Spec.ClientID}, "scope": {f.Spec.Scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCode{}, usage.NewFailureError("network", "The provider request failed.")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return DeviceCode{}, usage.NewFailureError("network", "The provider request failed.")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// Redirects are never followed; rejected explicitly (SPEC §10).
		return DeviceCode{}, usage.NewFailureError("redirect_refused", "The provider attempted an unsafe redirect.")
	}
	body, err := security.ReadResponseBounded(resp, f.effectiveMaxBytes())
	if err != nil {
		return DeviceCode{}, usage.NewFailureError("network", "The provider request failed.")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeviceCode{}, usage.NewFailureError("provider_status",
			fmt.Sprintf("The device login endpoint is unavailable (HTTP %d).", resp.StatusCode))
	}

	obj, err := parseDeviceObject(body)
	if err != nil {
		return DeviceCode{}, unsupportedResponse()
	}

	var deviceCode, userCode, verificationURI string
	var expiresIn, interval int
	ok := jsonString(obj["device_code"], &deviceCode) &&
		jsonString(obj["user_code"], &userCode) &&
		jsonString(obj["verification_uri"], &verificationURI) &&
		jsonInt(obj["expires_in"], &expiresIn)
	if !ok {
		return DeviceCode{}, unsupportedResponse()
	}
	if err := security.ValidateOpaqueToken(deviceCode); err != nil {
		return DeviceCode{}, unsupportedResponse()
	}
	if !userCodePattern.MatchString(userCode) {
		return DeviceCode{}, unsupportedResponse()
	}
	if verificationURI != f.Spec.VerificationURI {
		// Exact-match allow-list (SPEC D9): a compromised device endpoint
		// must not steer the user to a phishing URL.
		return DeviceCode{}, unsupportedResponse()
	}
	if expiresIn < 1 || expiresIn > 1800 {
		return DeviceCode{}, unsupportedResponse()
	}
	if raw, present := obj["interval"]; present {
		if !jsonInt(raw, &interval) || interval < 1 || interval > 30 {
			return DeviceCode{}, unsupportedResponse()
		}
	} else {
		interval = 5
	}

	return DeviceCode{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: verificationURI,
		ExpiresIn:       time.Duration(expiresIn) * time.Second,
		Interval:        time.Duration(interval) * time.Second,
	}, nil
}

func (f *DeviceFlow) effectiveMaxBytes() int64 {
	if f.MaxResponseBytes <= 0 {
		return security.MaxResponseBytes
	}
	return f.MaxResponseBytes
}

func unsupportedResponse() error {
	return usage.NewFailureError("unsupported_response", "The device flow returned an unsupported response.")
}

// parseDeviceObject requires a non-null, non-array JSON object.
func parseDeviceObject(data []byte) (map[string]json.RawMessage, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	trimmed := strings.TrimLeft(string(raw), " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("device response is not a JSON object")
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func jsonString(raw json.RawMessage, dst *string) bool {
	if len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func jsonInt(raw json.RawMessage, dst *int) bool {
	if len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}
