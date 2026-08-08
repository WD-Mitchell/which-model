//go:build !nousage

package credential

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestResolveChain(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	envSource := func(name string) usage.AuthSource {
		return usage.AuthSource{Kind: usage.AuthEnvVar, EnvVar: name}
	}
	fileSource := func(t *testing.T, content string, perm os.FileMode) usage.AuthSource {
		t.Helper()
		p := writeCredFile(t, t.TempDir(), "cred.json", content, perm)
		return usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{p}, JSONPath: "token"}
	}

	t.Run("env beats file", func(t *testing.T) { // case 1
		t.Setenv("WM_CHAIN_TOK", "envtok")
		file := fileSource(t, `{"token":"filetok"}`, 0o600)
		cred, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{envSource("WM_CHAIN_TOK"), file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "envtok" || cred.Source != usage.AuthEnvVar {
			t.Fatalf("ResolveChain() = %+v, want envtok from env", cred)
		}
		if len(warnings) != 0 {
			t.Fatalf("ResolveChain() warnings = %v, want none", warnings)
		}
	})

	t.Run("unsafe env skips to file", func(t *testing.T) { // case 2
		t.Setenv("WM_CHAIN_TOK", "bad\ntok")
		file := fileSource(t, `{"token":"filetok"}`, 0o600)
		cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{envSource("WM_CHAIN_TOK"), file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "filetok" || cred.Source != usage.AuthFile {
			t.Fatalf("ResolveChain() = %+v, want filetok from file", cred)
		}
	})

	t.Run("all unavailable", func(t *testing.T) { // case 3
		dir := t.TempDir()
		file := usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{filepath.Join(dir, "nope.json")}, JSONPath: "token"}
		cred, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{envSource("WM_CHAIN_TOK"), file}, nil)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("ResolveChain() error = %v, want ErrNotFound", err)
		}
		if cred.Token != "" {
			t.Fatalf("ResolveChain() credential = %+v, want zero", cred)
		}
		if len(warnings) != 0 {
			t.Fatalf("ResolveChain() warnings = %v, want none", warnings)
		}
	})

	t.Run("validate failure skips", func(t *testing.T) { // case 4
		t.Setenv("WM_CHAIN_TOK", "envtok")
		env := envSource("WM_CHAIN_TOK")
		env.Validate = func(ctx context.Context, cand usage.Credential, client *http.Client) error {
			return errors.New("identity gate denied")
		}
		file := fileSource(t, `{"token":"filetok"}`, 0o600)
		cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{env, file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "filetok" {
			t.Fatalf("ResolveChain() = %+v, want filetok", cred)
		}
	})

	t.Run("accepted first candidate short-circuits", func(t *testing.T) { // case 5
		t.Setenv("WM_CHAIN_TOK", "envtok")
		env := envSource("WM_CHAIN_TOK")
		env.Validate = func(ctx context.Context, cand usage.Credential, client *http.Client) error { return nil }
		dir := t.TempDir()
		bad := writeCredFile(t, dir, "bad.json", `{"token":`, 0o600) // malformed: would be a hard error if touched
		file := usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{bad}, JSONPath: "token"}
		cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{env, file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil (second source never touched)", err)
		}
		if cred.Token != "envtok" {
			t.Fatalf("ResolveChain() = %+v, want envtok", cred)
		}
	})

	t.Run("hard file error aborts", func(t *testing.T) { // case 6
		t.Setenv("WM_CHAIN_TOK", "envtok")
		dir := t.TempDir()
		bad := writeCredFile(t, dir, "bad.json", `{"token":`, 0o600)
		file := usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{bad}, JSONPath: "token"}
		_, _, err := ResolveChain(context.Background(), []usage.AuthSource{file, envSource("WM_CHAIN_TOK")}, nil)
		if code := failureCode(t, err); code != "credential_json" {
			t.Fatalf("ResolveChain() error code = %q, want credential_json", code)
		}
	})

	t.Run("winning file warning aggregated", func(t *testing.T) { // case 7
		p := writeCredFile(t, t.TempDir(), "cred.json", `{"token":"tok"}`, 0o644)
		file := usage.AuthSource{Kind: usage.AuthFile, FilePaths: []string{p}, JSONPath: "token"}
		_, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if len(warnings) != 1 {
			t.Fatalf("ResolveChain() warnings = %v, want exactly 1", warnings)
		}
		if !strings.Contains(warnings[0].Message, p) {
			t.Fatalf("warning %q does not name the winning file %q", warnings[0].Message, p)
		}
	})

	t.Run("env win has no warnings", func(t *testing.T) { // case 8
		t.Setenv("WM_CHAIN_TOK", "envtok")
		_, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{envSource("WM_CHAIN_TOK")}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("ResolveChain() warnings = %v, want none", warnings)
		}
	})

	t.Run("unimplemented kind skipped", func(t *testing.T) { // case 9
		t.Setenv("WM_CHAIN_TOK", "envtok")
		rpc := usage.AuthSource{Kind: usage.AuthSubprocessRPC}
		cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{rpc, envSource("WM_CHAIN_TOK")}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "envtok" {
			t.Fatalf("ResolveChain() = %+v, want envtok", cred)
		}
	})

	t.Run("canary in validate error", func(t *testing.T) { // case 10A
		t.Setenv("WM_CHAIN_TOK", canary)
		env := envSource("WM_CHAIN_TOK")
		env.Validate = func(ctx context.Context, cand usage.Credential, client *http.Client) error {
			return errors.New("bad " + canary)
		}
		file := fileSource(t, `{"token":"filetok"}`, 0o600)
		cred, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{env, file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "filetok" {
			t.Fatalf("ResolveChain() = %+v, want filetok", cred)
		}
		for _, w := range warnings {
			if strings.Contains(w.Message, canary) {
				t.Fatalf("warning %q leaks canary", w.Message)
			}
		}
	})

	t.Run("canary extra never warned", func(t *testing.T) { // case 10B
		p := writeCredFile(t, t.TempDir(), "cred.json", `{"token":"tok","account_id":"`+canary+`"}`, 0o644)
		file := usage.AuthSource{
			Kind:       usage.AuthFile,
			FilePaths:  []string{p},
			JSONPath:   "token",
			ExtraPaths: map[string]string{"account_id": "account_id"},
		}
		_, warnings, err := ResolveChain(context.Background(), []usage.AuthSource{file}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if len(warnings) != 1 {
			t.Fatalf("ResolveChain() warnings = %v, want exactly 1", warnings)
		}
		for _, w := range warnings {
			if strings.Contains(w.Message, canary) {
				t.Fatalf("warning %q leaks canary", w.Message)
			}
		}
	})

	t.Run("cookie kind skipped", func(t *testing.T) {
		t.Setenv("WM_CHAIN_TOK", "envtok")
		cookie := usage.AuthSource{Kind: usage.AuthBrowserCookie}
		cred, _, err := ResolveChain(context.Background(), []usage.AuthSource{cookie, envSource("WM_CHAIN_TOK")}, nil)
		if err != nil {
			t.Fatalf("ResolveChain() error = %v, want nil", err)
		}
		if cred.Token != "envtok" {
			t.Fatalf("ResolveChain() = %+v, want envtok", cred)
		}
	})
}
