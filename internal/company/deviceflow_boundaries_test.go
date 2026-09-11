//go:build !nousage

package company_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/copilot"
)

type nativeDeviceTransport func(*http.Request) (*http.Response, error)

func (f nativeDeviceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNativeManagedDeviceFlowBoundaries(t *testing.T) {
	if os.Getenv("COMPANY_POLICY_LIVE_TEST") != "1" {
		t.Skip("isolated CI enrollment required")
	}
	state, err := company.Load()
	if err != nil || !state.Managed || !state.Required {
		t.Fatal("CI enrollment not active", err)
	}
	spec := usage.OAuthSpec{ClientID: copilot.CopilotClientID, DeviceCodeURL: copilot.GitHubDeviceCodeURL,
		TokenURL: copilot.GitHubDeviceTokenURL, VerificationURI: "https://github.com/login/device"}
	for _, provider := range []string{"copilot", "forbidden", ""} {
		t.Run("provider="+provider, func(t *testing.T) {
			flow := credential.NewDeviceFlow(spec)
			if provider != "" {
				flow = credential.NewProviderDeviceFlow(provider, spec)
			}
			requests := 0
			flow.HTTPClient = &http.Client{Transport: nativeDeviceTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				body := `{"access_token":"SYNTHETIC_TOKEN"}`
				if r.URL.String() == spec.DeviceCodeURL {
					body = `{"device_code":"SYNTHETIC_DEVICE","user_code":"TEST-CODE","verification_uri":"https://github.com/login/device","expires_in":60}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			_, startErr := flow.Start(context.Background())
			_, pollErr := flow.Poll(context.Background(), credential.DeviceCode{DeviceCode: "SYNTHETIC_DEVICE", ExpiresIn: time.Minute})
			if provider == "copilot" {
				if startErr != nil || pollErr != nil || requests != 2 {
					t.Fatalf("allowed provider: Start=%v Poll=%v requests=%d", startErr, pollErr, requests)
				}
			} else {
				var denied *company.Error
				if !errors.As(startErr, &denied) || !errors.As(pollErr, &denied) || requests != 0 {
					t.Fatalf("refused flow: Start=%v Poll=%v requests=%d", startErr, pollErr, requests)
				}
			}
		})
	}
}
