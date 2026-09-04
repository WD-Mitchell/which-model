//go:build !nousage

package credential

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// writeCredFile creates a fixture with a deterministic mode (Chmod defeats
// umask so the asserted mode is exact).
func writeCredFile(t *testing.T, dir, name, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

func failureCode(t *testing.T, err error) string {
	t.Helper()
	f, ok := usage.AsFailure(err)
	if !ok {
		t.Fatalf("error %v is not a *usage.FailureError", err)
	}
	return f.Code
}

func TestFileResolver(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	t.Run("nonexistent path", func(t *testing.T) { // case 1
		dir := t.TempDir()
		r := &FileResolver{Paths: []string{filepath.Join(dir, "nope.json")}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("nested json path", func(t *testing.T) { // case 2
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"tokens":{"access_token":"tok"}}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "tokens.access_token"}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Token != "tok" || cred.Source != usage.AuthFile {
			t.Fatalf("Resolve() = %+v, want token tok source file", cred)
		}
		if cred.Mode != 0o600 {
			t.Fatalf("Resolve() Mode = %o, want 600", cred.Mode)
		}
	})

	t.Run("flat json path", func(t *testing.T) { // case 3
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"access_token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "access_token"}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Token != "tok" {
			t.Fatalf("Resolve() token = %q, want tok", cred.Token)
		}
	})

	t.Run("json path missing", func(t *testing.T) { // case 4
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"tokens":{}}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "tokens.access_token"}
		_, err := r.Resolve(context.Background())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) { // case 5
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"tokens":`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "credential_json" {
			t.Fatalf("Resolve() error code = %q, want credential_json", code)
		}
	})

	t.Run("json array", func(t *testing.T) { // case 6
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `["a","b"]`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "credential_json" {
			t.Fatalf("Resolve() error code = %q, want credential_json", code)
		}
	})

	t.Run("unsafe token", func(t *testing.T) { // case 7
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"bad\ttok"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "unsafe_credential" {
			t.Fatalf("Resolve() error code = %q, want unsafe_credential", code)
		}
	})

	t.Run("empty token value", func(t *testing.T) { // case 8
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":""}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("oversized file", func(t *testing.T) { // case 9
		dir := t.TempDir()
		p := writeCredFile(t, dir, "big.json", strings.Repeat("{", 1_100_000), 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "credential_file" {
			t.Fatalf("Resolve() error code = %q, want credential_file", code)
		}
		if int64(len(strings.Repeat("{", 1_100_000))) <= security.MaxCredentialBytes {
			t.Fatal("fixture not actually oversized")
		}
	})

	t.Run("missing then valid path", func(t *testing.T) { // case 10
		dir := t.TempDir()
		valid := writeCredFile(t, dir, "cred.json", `{"token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{filepath.Join(dir, "nope.json"), valid}, JSONPath: "token"}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Token != "tok" {
			t.Fatalf("Resolve() token = %q, want tok", cred.Token)
		}
	})

	t.Run("unreadable file", func(t *testing.T) { // case 11
		if os.Geteuid() == 0 {
			t.Skip("running as root; chmod 0000 is not a read error")
		}
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok"}`, 0o000)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "credential_file" {
			t.Fatalf("Resolve() error code = %q, want credential_file", code)
		}
	})

	t.Run("canary in invalid token", func(t *testing.T) { // case 12
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token": "`+canary+`\n"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "unsafe_credential" {
			t.Fatalf("Resolve() error code = %q, want unsafe_credential", code)
		}
		if strings.Contains(err.Error(), canary) {
			t.Fatalf("error %q leaks canary", err)
		}
	})
}

func TestFileResolverExpandedCandidates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	override := t.TempDir()
	t.Setenv("WHICH_MODEL_TEST_CREDENTIAL_DIR", override)
	t.Setenv("WHICH_MODEL_TEST_EMPTY_DIR", "")
	// Setting then unsetting ensures the caller environment is restored.
	t.Setenv("WHICH_MODEL_TEST_MISSING_DIR", "")
	if err := os.Unsetenv("WHICH_MODEL_TEST_MISSING_DIR"); err != nil {
		t.Fatal(err)
	}
	writeCredFile(t, home, "auth.json", `{"token":"home-synthetic-token"}`, 0o600)
	absolute := writeCredFile(t, override, "auth.json", `{"token":"override-synthetic-token"}`, 0o600)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, absolute)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		paths []string
		want  string
	}{
		{"home", []string{"~/auth.json"}, "home-synthetic-token"},
		{"environment", []string{"$WHICH_MODEL_TEST_CREDENTIAL_DIR/auth.json"}, "override-synthetic-token"},
		{"braced environment", []string{"${WHICH_MODEL_TEST_CREDENTIAL_DIR}/auth.json"}, "override-synthetic-token"},
		{"missing fallback", []string{"$WHICH_MODEL_TEST_MISSING_DIR/auth.json", "~/auth.json"}, "home-synthetic-token"},
		{"empty fallback", []string{"$WHICH_MODEL_TEST_EMPTY_DIR/auth.json", "~/auth.json"}, "home-synthetic-token"},
		{"precedence", []string{"$WHICH_MODEL_TEST_CREDENTIAL_DIR/auth.json", "~/auth.json"}, "override-synthetic-token"},
		{"absolute", []string{absolute}, "override-synthetic-token"},
		{"relative", []string{relative}, "override-synthetic-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cred, err := (&FileResolver{Paths: tc.paths, JSONPath: "token"}).Resolve(context.Background())
			if err != nil || cred.Token != tc.want || cred.Source != usage.AuthFile {
				t.Fatalf("expected file credential; got source=%v err=%v", cred.Source, err)
			}
		})
	}
}

func TestFileResolverExpansionIsLiteral(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "$(touch marker)-`id`-$UNEXPANDED")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeCredFile(t, dir, "auth.json", `{"token":"literal-synthetic-token"}`, 0o600)
	t.Setenv("WHICH_MODEL_TEST_LITERAL_DIR", dir)
	cred, err := (&FileResolver{Paths: []string{"$WHICH_MODEL_TEST_LITERAL_DIR/auth.json"}, JSONPath: "token"}).Resolve(context.Background())
	if err != nil || cred.Token != "literal-synthetic-token" {
		t.Fatalf("literal environment path must resolve without recursive expansion: %v", err)
	}
}

func TestExpandCredentialPathMissingVariables(t *testing.T) {
	t.Setenv("WHICH_MODEL_TEST_MISSING_EXPANSION", "")
	for _, variable := range []string{"$WHICH_MODEL_TEST_MISSING_EXPANSION", "${WHICH_MODEL_TEST_MISSING_EXPANSION}"} {
		path, usable := expandCredentialPath(variable + "/auth.json")
		if usable || path != "" {
			t.Fatal("missing variable must invalidate the candidate, never select a root path")
		}
	}
	if err := os.Unsetenv("WHICH_MODEL_TEST_MISSING_EXPANSION"); err != nil {
		t.Fatal(err)
	}
	if _, usable := expandCredentialPath("$WHICH_MODEL_TEST_MISSING_EXPANSION/auth.json"); usable {
		t.Fatal("unset variable must invalidate candidate")
	}
}
