//go:build !nousage

package whichmodel

import (
	"errors"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

const tokenCanary = "ghp_CANARY-TOKEN-9f3a"

func TestAuthTokenNeverRendered(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: tokenCanary}, nil
	}
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), tokenCanary) || !strings.Contains(out.String(), Fingerprint(tokenCanary)) {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestAuthAccountHiddenByDefault(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", Account: "secret-account-42"}, nil
	}
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, JSON: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "secret-account-42") || strings.Contains(out.String(), `"account"`) {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestAuthAccountShownWithFlag(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", Account: "secret-account-42"}, nil
	}
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, JSON: true, ShowIdentity: true}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"account": "secret-account-42"`) {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestAuthTextRedaction(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{Source: usage.SourceOAuth, Secret: tokenCanary, Account: "secret-account-42"}, nil
	}
	var out, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), tokenCanary) || strings.Contains(out.String(), "secret-account-42") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestAuthFailureRedaction(t *testing.T) {
	old := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = old })
	resolveFirstFunc = func(string) (AuthResolved, error) {
		return AuthResolved{}, errors.New("resolver leaked " + tokenCanary)
	}
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}}, &out, &errOut)
	if strings.Contains(errOut.String(), tokenCanary) || strings.Contains(err.Error(), tokenCanary) {
		t.Fatalf("canary leaked: err=%q stderr=%q", err.Error(), errOut.String())
	}
}
