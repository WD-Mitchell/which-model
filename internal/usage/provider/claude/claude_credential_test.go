//go:build !nousage

package claude

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const canaryToken = "canary-secret-token-123"

// writeCred writes credJSON to dir/path with the given mode, creating parent
// dirs, and returns the absolute path.
func writeCred(t *testing.T, dir, rel string, mode os.FileMode, credJSON string) string {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(credJSON), mode); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
	if err := os.Chmod(abs, mode); err != nil {
		t.Fatalf("chmod %s: %v", abs, err)
	}
	return abs
}

func TestLoadFileCredential(t *testing.T) {
	now := time.Now()

	t.Run("nested claudeAiOauth accessToken + expiresAt", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(now.Add(60*time.Second).UnixMilli())+`}}`)
		fc, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.Token != canaryToken {
			t.Errorf("Token = %q, want %q", fc.Token, canaryToken)
		}
		if fc.ExpiresAt == nil || !fc.ExpiresAt.Equal(now.Add(60*time.Second).Truncate(time.Millisecond)) {
			t.Errorf("ExpiresAt = %v, want ~%v", fc.ExpiresAt, now.Add(60*time.Second))
		}
		if fc.BroadPermissions {
			t.Errorf("BroadPermissions = true, want false for 0600")
		}
	})

	t.Run("snake keys access_token + expires_at", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"claudeAiOauth":{"access_token":"`+canaryToken+`","expires_at":`+itoa(now.Add(60*time.Second).UnixMilli())+`}}`)
		fc, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.Token != canaryToken {
			t.Errorf("Token = %q, want %q", fc.Token, canaryToken)
		}
	})

	t.Run("plain file only, flat accessToken", func(t *testing.T) {
		dir := t.TempDir()
		dot := filepath.Join(dir, ".claude/.credentials.json")
		plain := writeCred(t, dir, ".claude/credentials.json", 0o600,
			`{"accessToken":"`+canaryToken+`"}`)
		fc, err := LoadFileCredential(dot, plain, now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.Token != canaryToken {
			t.Errorf("Token = %q, want %q", fc.Token, canaryToken)
		}
	})

	t.Run("oauth object", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"oauth":{"accessToken":"`+canaryToken+`"}}`)
		fc, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.Token != canaryToken {
			t.Errorf("Token = %q, want %q", fc.Token, canaryToken)
		}
	})

	t.Run("neither file exists", func(t *testing.T) {
		dir := t.TempDir()
		fc, err := LoadFileCredential(filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.Token != "" || fc.ExpiresAt != nil || fc.BroadPermissions {
			t.Errorf("FileCredential = %+v, want zero value", fc)
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600, `{bad`)
		_, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		assertErrorCode(t, err, "credential_json", "The credential file is not valid JSON.", "credential_json")
	})

	t.Run("empty object", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600, `{}`)
		_, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		assertErrorCode(t, err, "unsafe_credential", "The Claude access token is missing or unsafe.", "unsafe_credential")
	})

	t.Run("expired epoch seconds", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":1}}`)
		_, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), time.UnixMilli(10_000))
		assertErrorCode(t, err, "expired_credential", "The Claude access token is expired.", "expired_credential")
	})

	t.Run("expired milliseconds", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"claudeAiOauth":{"accessToken":"`+canaryToken+`","expiresAt":`+itoa(now.Add(-1000*time.Millisecond).UnixMilli())+`}}`)
		_, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		assertErrorCode(t, err, "expired_credential", "The Claude access token is expired.", "expired_credential")
	})

	t.Run("broad permissions", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o644,
			`{"claudeAiOauth":{"accessToken":"`+canaryToken+`"}}`)
		fc, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if !fc.BroadPermissions {
			t.Errorf("BroadPermissions = false, want true for 0644")
		}
	})

	t.Run("restrictive permissions", func(t *testing.T) {
		dir := t.TempDir()
		dot := writeCred(t, dir, ".claude/.credentials.json", 0o600,
			`{"claudeAiOauth":{"accessToken":"`+canaryToken+`"}}`)
		fc, err := LoadFileCredential(dot, filepath.Join(dir, ".claude/credentials.json"), now)
		if err != nil {
			t.Fatalf("LoadFileCredential: %v", err)
		}
		if fc.BroadPermissions {
			t.Errorf("BroadPermissions = true, want false for 0600")
		}
	})
}

// assertErrorCode asserts err is a *Error with the given code and message,
// and that the message never contains credential material.
func assertErrorCode(t *testing.T, err error, code, message, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T %v, want *claude.Error", err, err)
	}
	if e.Code != wantCode {
		t.Errorf("Code = %q, want %q", e.Code, wantCode)
	}
	if e.Message != message {
		t.Errorf("Message = %q, want %q", e.Message, message)
	}
	if strings.Contains(e.Message, canaryToken) {
		t.Errorf("error message contains credential material: %q", e.Message)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
