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
