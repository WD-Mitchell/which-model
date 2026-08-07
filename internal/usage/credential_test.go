package usage

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAuthKindString(t *testing.T) {
	cases := []struct {
		kind AuthKind
		want string
	}{
		{AuthEnvVar, "env"},
		{AuthFile, "file"},
		{AuthKeychainGeneric, "keychain-generic"},
		{AuthKeychainInternet, "keychain-internet"},
		{AuthBrowserCookie, "cookie"},
		{AuthCLIShellOut, "cli"},
		{AuthSubprocessRPC, "rpc"},
		{AuthOAuthDeviceFlow, "oauth-device"},
		{AuthOAuthRefreshGrant, "oauth-refresh"},
		{AuthAWSSigV4, "aws-sigv4"},
		{AuthVolcengineAKSK, "volcengine-aksk"},
		{AuthGRPCWebToken, "grpc-web-token"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("AuthKind(%d).String() = %q, want %q", int(tc.kind), got, tc.want)
		}
	}
	if got := AuthKind(999).String(); got != "unknown" {
		t.Errorf("AuthKind(999).String() = %q, want unknown", got)
	}
}

func TestCredentialStringRedaction(t *testing.T) {
	s := Credential{Token: "sekrit", Source: AuthFile}.String()
	if !strings.Contains(s, "<redacted>") {
		t.Errorf("Credential.String() = %q, must contain <redacted>", s)
	}
	if strings.Contains(s, "sekrit") {
		t.Errorf("Credential.String() leaks token: %q", s)
	}
}

func TestCredentialStringCanary(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"
	s := Credential{
		Token: canary,
		Extra: map[string]string{"account_id": canary},
	}.String()
	if strings.Contains(s, canary) {
		t.Errorf("Credential.String() leaks canary token/Extra: %q", s)
	}
}

func TestFailureErrorFormat(t *testing.T) {
	if got := NewFailureError("timeout", "call timed out").Error(); got != "timeout: call timed out" {
		t.Errorf("Error() = %q, want \"timeout: call timed out\"", got)
	}
}

func TestAsFailure(t *testing.T) {
	f, ok := AsFailure(NewFailureError("login_required", "no cred"))
	if !ok || f.Code != "login_required" || f.Message != "no cred" {
		t.Errorf("AsFailure direct = (%v, %v), want ({login_required no cred}, true)", f, ok)
	}
	wrapped := fmt.Errorf("wrap: %w", NewFailureError("network", "dns"))
	f, ok = AsFailure(wrapped)
	if !ok || f.Code != "network" {
		t.Errorf("AsFailure wrapped = (%v, %v), want ({network ...}, true)", f, ok)
	}
	f, ok = AsFailure(errors.New("plain"))
	if ok || f != (Failure{}) {
		t.Errorf("AsFailure plain = (%v, %v), want ({}, false)", f, ok)
	}
}
