//go:build !nousage

package whichmodel

import (
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestAuthExpiryTextGolden(t *testing.T) {
	oldResolver := resolveFirstFunc
	oldNow := nowFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver; nowFunc = oldNow })
	nowFunc = func() time.Time { return time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC) }
	future := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	resolveFirstFunc = func(id string) (AuthResolved, error) {
		switch id {
		case "claude":
			return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &future}, nil
		case "codex":
			return AuthResolved{Source: usage.SourceOAuth, Secret: "other"}, nil
		default:
			return AuthResolved{}, errNoCredential
		}
	}
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude", "codex", "copilot"}}, &out, &errOut)
	if ExitCodeFor(err) != 5 {
		t.Fatalf("exit = %d, err = %v", ExitCodeFor(err), err)
	}
	want := "claude   ok       oauth   " + Fingerprint("tok") + "   (expires 2026-09-01T00:00:00Z)\n" +
		"codex    ok       oauth   " + Fingerprint("other") + "\n" +
		"copilot  missing  -       -            -    run: which-model auth login copilot\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestAuthExpiredRendering(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	past := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", ExpiresAt: &past}, nil }
	var out, errOut strings.Builder
	err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}}, &out, &errOut)
	if ExitCodeFor(err) != 5 || !strings.Contains(out.String(), "(expired 2026-07-01T00:00:00Z)") {
		t.Fatalf("err = %v, out = %q", err, out.String())
	}
}

func TestAuthAccountColumn(t *testing.T) {
	oldResolver := resolveFirstFunc
	t.Cleanup(func() { resolveFirstFunc = oldResolver })
	resolveFirstFunc = func(string) (AuthResolved, error) { return AuthResolved{Source: usage.SourceOAuth, Secret: "tok", Account: "user@x"}, nil }
	var shown, hidden, errOut strings.Builder
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}, ShowIdentity: true}, &shown, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(shown.String(), "(account user@x)\n") {
		t.Fatalf("shown = %q", shown.String())
	}
	if err := RunAuthStatus(AuthStatusArgs{Providers: []string{"claude"}}, &hidden, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hidden.String(), "(account") {
		t.Fatalf("hidden = %q", hidden.String())
	}
}
