//go:build !nousage

package cursor

import "testing"

func TestCursorLoginURLAcceptsCursorAuthorizationURL(t *testing.T) {
	got := cursorLoginURL("Open https://authenticator.cursor.sh/login?challenge=test). in your browser")
	if got != "https://authenticator.cursor.sh/login?challenge=test" {
		t.Fatalf("cursorLoginURL() = %q", got)
	}
}

func TestCursorLoginURLRejectsLookalikeHost(t *testing.T) {
	for _, output := range []string{
		"https://cursor.sh.example.com/login",
		"https://cursor.com.evil.invalid/login",
		"http://cursor.com/login",
	} {
		if got := cursorLoginURL(output); got != "" {
			t.Errorf("cursorLoginURL(%q) = %q, want empty", output, got)
		}
	}
}
