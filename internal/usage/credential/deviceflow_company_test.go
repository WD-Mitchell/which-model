//go:build !nousage

package credential

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

type companyFlowTransport func(*http.Request) (*http.Response, error)

func (f companyFlowTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func companyFlowSpec() usage.OAuthSpec {
	return usage.OAuthSpec{ClientID: "synthetic", DeviceCodeURL: "https://github.com/login/device/code",
		TokenURL: "https://github.com/login/oauth/access_token", VerificationURI: "https://github.com/login/device"}
}

func TestDeviceFlowCompanyRequestBoundaries(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	for _, tc := range []struct {
		name, provider            string
		managed, missing, allowed bool
	}{
		{"personal unbound", "", false, false, true},
		{"managed allowed", "copilot", true, false, true},
		{"managed unbound", "", true, false, false},
		{"managed forbidden", "claude", true, false, false},
		{"required policy missing", "copilot", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := company.Defaults()
			p.AllowedProviders = []string{"copilot"}
			readCompanyPolicy = func() (company.Snapshot, error) {
				if tc.missing {
					return company.Snapshot{}, &company.Error{Reason: "required policy missing"}
				}
				return company.Snapshot{Managed: tc.managed, Policy: &p}, nil
			}
			flow := NewDeviceFlow(companyFlowSpec())
			if tc.provider != "" {
				flow = NewProviderDeviceFlow(tc.provider, companyFlowSpec())
			}
			calls := 0
			flow.HTTPClient = &http.Client{Transport: companyFlowTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `{"access_token":"SYNTHETIC_TOKEN"}`
				if r.URL.Path == "/login/device/code" {
					body = `{"device_code":"SYNTHETIC_DEVICE","user_code":"TEST-CODE","verification_uri":"https://github.com/login/device","expires_in":60}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			_, startErr := flow.Start(context.Background())
			_, pollErr := flow.Poll(context.Background(), DeviceCode{DeviceCode: "SYNTHETIC_DEVICE", ExpiresIn: time.Minute})
			if tc.allowed {
				if startErr != nil || pollErr != nil || calls != 2 {
					t.Fatalf("allowed flow: Start=%v Poll=%v requests=%d", startErr, pollErr, calls)
				}
			} else {
				var denied *company.Error
				if !errors.As(startErr, &denied) || !errors.As(pollErr, &denied) || calls != 0 {
					t.Fatalf("refused flow: Start=%v Poll=%v requests=%d", startErr, pollErr, calls)
				}
			}
		})
	}
}

func TestDeviceFlowCompanyRevocationBetweenPolls(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	for _, missing := range []bool{false, true} {
		for _, response := range []string{"authorization_pending", "slow_down"} {
			p := company.Defaults()
			p.AllowedProviders = []string{"copilot"}
			revoked := false
			readCompanyPolicy = func() (company.Snapshot, error) {
				if revoked && missing {
					return company.Snapshot{}, &company.Error{Reason: "required policy missing"}
				}
				return company.Snapshot{Managed: true, Policy: &p}, nil
			}
			flow := NewProviderDeviceFlow("copilot", companyFlowSpec())
			calls := 0
			flow.HTTPClient = &http.Client{Transport: companyFlowTransport(func(*http.Request) (*http.Response, error) {
				calls++
				body := `{"error":"` + response + `"}`
				if calls > 1 {
					body = `{"access_token":"SYNTHETIC_TOKEN"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			flow.Sleep = func(time.Duration) { revoked = true; p.AllowedProviders = nil }
			token, err := flow.Poll(context.Background(), DeviceCode{DeviceCode: "SYNTHETIC_DEVICE", ExpiresIn: time.Minute, Interval: time.Second})
			var denied *company.Error
			if !errors.As(err, &denied) || calls != 1 || token != "" {
				t.Errorf("missing=%v response=%s: requests=%d token returned=%v err=%v", missing, response, calls, token != "", err)
			}
		}
	}
}
