//go:build !nousage

package copilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
)

// userCodePattern is the GitHub device user_code format (copilot.mjs:130-155).
var userCodePattern = regexp.MustCompile(`^[A-Z0-9-]{4,32}$`)

// deviceVerificationURI is the EXACT allow-listed verification URI
// (copilot.mjs:130-155; SPEC §2.9).
const deviceVerificationURI = "https://github.com/login/device"

// DeviceFlow is the validated startDeviceFlow result (copilot.mjs:130-155).
type DeviceFlow struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string // always "https://github.com/login/device"
	ExpiresIn       int    // 1..1800
	Interval        int    // 1..30
}

// StartDeviceFlow is the port of startDeviceFlow (copilot.mjs:122-155):
// POST GitHubDeviceCodeURL, headers {Accept: application/json, Content-Type:
// application/x-www-form-urlencoded}, body client_id=CopilotClientID&scope=read:user.
// Non-200 → Error per mapStatus("GitHub device login", status). Validation
// failures → Error{Code: "unsupported_response", Message: "GitHub returned an
// unsupported device-login response."} (device_code opaque shape, user_code
// ^[A-Z0-9-]{4,32}$, verification_uri href == "https://github.com/login/device",
// expires_in 1..1800, interval (default 5) 1..30).
func StartDeviceFlow(ctx context.Context, client *http.Client) (DeviceFlow, error) {
	form := url.Values{}
	form.Set("client_id", CopilotClientID)
	form.Set("scope", "read:user")
	status, raw, err := doRequest(ctx, client, http.MethodPost, GitHubDeviceCodeURL, []string{GitHubDeviceCodeURL}, map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/x-www-form-urlencoded",
	}, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceFlow{}, err
	}
	if status != http.StatusOK {
		return DeviceFlow{}, mapStatus("GitHub device login", status)
	}

	reject := func() (DeviceFlow, error) {
		return DeviceFlow{}, &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported device-login response."}
	}

	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return reject()
	}

	// device_code must pass the opaque-token shape.
	var deviceCode string
	if raw, ok := value["device_code"]; !ok || json.Unmarshal(raw, &deviceCode) != nil || security.ValidateOpaqueToken(deviceCode) != nil {
		return reject()
	}

	// user_code ^[A-Z0-9-]{4,32}$.
	var userCode string
	if raw, ok := value["user_code"]; !ok || json.Unmarshal(raw, &userCode) != nil || !userCodePattern.MatchString(userCode) {
		return reject()
	}

	// verification_uri: parses and its href equals the pinned value EXACTLY
	// (scheme+host+path; trailing slash, extra path, query, fragment, or
	// userinfo all reject).
	var verificationURI string
	if raw, ok := value["verification_uri"]; !ok || json.Unmarshal(raw, &verificationURI) != nil {
		return reject()
	}
	u, err := url.Parse(verificationURI)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.Path != "/login/device" ||
		u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return reject()
	}

	// expires_in required, finite 1..1800.
	expiresRaw, ok := value["expires_in"]
	if !ok || isNull(expiresRaw) {
		return reject()
	}
	expiresIn, ok := boundedInt(expiresRaw, 1, 1800)
	if !ok {
		return reject()
	}

	// interval (default 5) finite 1..30.
	interval := 5
	if raw, present := value["interval"]; present && !isNull(raw) {
		if interval, ok = boundedInt(raw, 1, 30); !ok {
			return reject()
		}
	}

	return DeviceFlow{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: deviceVerificationURI,
		ExpiresIn:       expiresIn,
		Interval:        interval,
	}, nil
}

// boundedInt parses a JSON number or numeric string into an int within
// [min, max]; ok=false otherwise.
func boundedInt(raw json.RawMessage, min, max int) (int, bool) {
	n, ok := finiteNumber(raw)
	if !ok || n < float64(min) || n > float64(max) {
		return 0, false
	}
	return int(n), true
}

// PollOptions injects the clock for tests (mirrors copilot.mjs pollDeviceFlow's
// now/sleep parameters); nil opts use time.Now/time.Sleep.
type PollOptions struct {
	Now   func() time.Time
	Sleep func(d time.Duration)
}

// PollDeviceFlow is the port of pollDeviceFlow (copilot.mjs:157-195): local
// deadline now+ExpiresIn*1000; never requests at/after the deadline; per
// iteration sleeps min(Interval*1000, remaining); POST GitHubDeviceTokenURL
// with the form headers plus device_code and
// grant_type=urn:ietf:params:oauth:grant-type:device_code. access_token →
// opaque check → returned. error: authorization_pending → continue;
// slow_down → Interval += 5; access_denied → Error{Code: "access_denied",
// Message: "GitHub device login was denied or cancelled."}; expired_token →
// Error{Code: "device_expired", Message: "GitHub device login expired."};
// other → Error{Code: "unsupported_response", Message: "GitHub returned an
// unsupported device-login response."}. Deadline exit → device_expired.
func PollDeviceFlow(ctx context.Context, client *http.Client, flow DeviceFlow, opts *PollOptions) (string, error) {
	now := time.Now
	sleep := time.Sleep
	if opts != nil {
		if opts.Now != nil {
			now = opts.Now
		}
		if opts.Sleep != nil {
			sleep = opts.Sleep
		}
	}

	interval := flow.Interval
	deadline := now().Add(time.Duration(flow.ExpiresIn) * time.Second)

	for {
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			break
		}
		wait := time.Duration(interval) * time.Second
		if wait > remaining {
			wait = remaining
		}
		sleep(wait)
		if !now().Before(deadline) {
			break
		}

		form := url.Values{}
		form.Set("client_id", CopilotClientID)
		form.Set("device_code", flow.DeviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		status, raw, err := doRequest(ctx, client, http.MethodPost, GitHubDeviceTokenURL, []string{GitHubDeviceTokenURL}, map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/x-www-form-urlencoded",
		}, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		if status != http.StatusOK {
			return "", mapStatus("GitHub device login", status)
		}

		var value map[string]json.RawMessage
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
		}

		if tokRaw, ok := value["access_token"]; ok {
			var token string
			if json.Unmarshal(tokRaw, &token) != nil || security.ValidateOpaqueToken(token) != nil {
				return "", &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported device-login response."}
			}
			return token, nil
		}

		var errCode string
		_ = json.Unmarshal(value["error"], &errCode)
		switch errCode {
		case "", "authorization_pending":
			// continue silently
		case "slow_down":
			interval += 5 // unbounded, .mjs verbatim
		case "access_denied":
			return "", &Error{Code: "access_denied", Message: "GitHub device login was denied or cancelled."}
		case "expired_token":
			return "", &Error{Code: "device_expired", Message: "GitHub device login expired."}
		default:
			return "", &Error{Code: "unsupported_response", Message: "GitHub returned an unsupported device-login response."}
		}
	}
	return "", &Error{Code: "device_expired", Message: "GitHub device login expired."}
}
