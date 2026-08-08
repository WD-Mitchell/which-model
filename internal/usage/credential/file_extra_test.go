//go:build !nousage

package credential

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFileResolverExtrasAndExpiry(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	t.Run("extra extracted", func(t *testing.T) { // case 1
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"tokens":{"access_token":"tok","account_id":"acct-7"}}`, 0o600)
		r := &FileResolver{
			Paths:      []string{p},
			JSONPath:   "tokens.access_token",
			ExtraPaths: map[string]string{"account_id": "tokens.account_id"},
		}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Extra["account_id"] != "acct-7" {
			t.Fatalf("Resolve() Extra = %v, want account_id acct-7", cred.Extra)
		}
	})

	t.Run("extra path missing omitted", func(t *testing.T) { // case 2
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"tokens":{"access_token":"tok"}}`, 0o600)
		r := &FileResolver{
			Paths:      []string{p},
			JSONPath:   "tokens.access_token",
			ExtraPaths: map[string]string{"account_id": "tokens.account_id"},
		}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if _, ok := cred.Extra["account_id"]; ok {
			t.Fatalf("Resolve() Extra = %v, want account_id omitted", cred.Extra)
		}
	})

	t.Run("future expiry resolves", func(t *testing.T) { // case 3
		dir := t.TempDir()
		exp := time.Now().Add(3600 * time.Second).Unix()
		p := writeCredFile(t, dir, "cred.json", fmt.Sprintf(`{"token":"tok","expires_at":%d}`, exp), 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token", ExpiryPath: "expires_at"}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Token != "tok" {
			t.Fatalf("Resolve() token = %q, want tok", cred.Token)
		}
	})

	t.Run("past expiry", func(t *testing.T) { // case 4
		dir := t.TempDir()
		exp := time.Now().Add(-60 * time.Second).Unix()
		p := writeCredFile(t, dir, "cred.json", fmt.Sprintf(`{"token":"tok","expires_at":%d}`, exp), 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token", ExpiryPath: "expires_at"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "expired_credential" {
			t.Fatalf("Resolve() error code = %q, want expired_credential", code)
		}
	})

	t.Run("unparseable expiry", func(t *testing.T) { // case 5
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok","expires_at":"soon"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token", ExpiryPath: "expires_at"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "expired_credential" {
			t.Fatalf("Resolve() error code = %q, want expired_credential", code)
		}
	})

	t.Run("ms epoch heuristic", func(t *testing.T) { // case 6
		// 1700000000000 ms == 2023-11-14T22:13:20Z. The >10_000_000_000
		// heuristic must read it as MILLISECONDS (a seconds reading would
		// be year ~55857 and therefore not expired); 2023 is in the past,
		// so the fail-safe (SPEC §3) reports expired_credential. [The
		// TASKS.md table says "resolves" for this row, which is
		// unreachable with a real clock in 2026 — the ms treatment is
		// still observable: seconds-interpretation would resolve.]
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok","expires_at":1700000000000}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token", ExpiryPath: "expires_at"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "expired_credential" {
			t.Fatalf("Resolve() error code = %q, want expired_credential (ms heuristic → 2023 → past)", code)
		}
	})

	t.Run("missing expiry path fails closed", func(t *testing.T) {
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token", ExpiryPath: "expires_at"}
		_, err := r.Resolve(context.Background())
		if code := failureCode(t, err); code != "expired_credential" {
			t.Fatalf("Resolve() error code = %q, want expired_credential (fail-safe)", code)
		}
	})
}

func TestFileResolverWarnings(t *testing.T) {
	const canary = "canary-9f3a2b1c4d5e6f78"

	t.Run("broad permissions warned", func(t *testing.T) { // case 7
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok"}`, 0o644)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Token != "tok" {
			t.Fatalf("Resolve() token = %q, want tok", cred.Token)
		}
		warnings := r.Warnings()
		if len(warnings) != 1 {
			t.Fatalf("Warnings() = %v, want exactly 1 entry", warnings)
		}
		mode := "-rw-r--r--"
		if !strings.Contains(warnings[0], p) || !strings.Contains(warnings[0], mode) {
			t.Fatalf("Warnings()[0] = %q, want it to contain path %q and mode %q", warnings[0], p, mode)
		}
	})

	t.Run("clean permissions no warning", func(t *testing.T) { // case 8
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{p}, JSONPath: "token"}
		if _, err := r.Resolve(context.Background()); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if warnings := r.Warnings(); len(warnings) != 0 {
			t.Fatalf("Warnings() = %v, want none", warnings)
		}
	})

	t.Run("winning file only warned", func(t *testing.T) { // case 9
		dir := t.TempDir()
		broad := writeCredFile(t, dir, "broad.json", `{"token":"tok"}`, 0o644)
		clean := writeCredFile(t, dir, "clean.json", `{"token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{broad, clean}, JSONPath: "token"}
		if _, err := r.Resolve(context.Background()); err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		warnings := r.Warnings()
		if len(warnings) != 1 {
			t.Fatalf("Warnings() = %v, want exactly 1 entry (winning file only)", warnings)
		}
		if !strings.Contains(warnings[0], broad) {
			t.Fatalf("Warnings()[0] = %q, want it to name the winning file %q", warnings[0], broad)
		}
	})

	t.Run("warnings reset per resolve", func(t *testing.T) {
		dir := t.TempDir()
		broad := writeCredFile(t, dir, "broad.json", `{"token":"tok"}`, 0o644)
		clean := writeCredFile(t, dir, "clean.json", `{"token":"tok"}`, 0o600)
		r := &FileResolver{Paths: []string{clean}, JSONPath: "token"}
		if _, err := r.Resolve(context.Background()); err != nil {
			t.Fatalf("first Resolve() error = %v", err)
		}
		r.Paths = []string{broad}
		if _, err := r.Resolve(context.Background()); err != nil {
			t.Fatalf("second Resolve() error = %v", err)
		}
		if warnings := r.Warnings(); len(warnings) != 1 || !strings.Contains(warnings[0], broad) {
			t.Fatalf("Warnings() = %v, want exactly the second resolve's warning", warnings)
		}
	})

	t.Run("canary extra never warned", func(t *testing.T) { // case 10
		dir := t.TempDir()
		p := writeCredFile(t, dir, "cred.json", `{"token":"tok","account_id":"`+canary+`"}`, 0o644)
		r := &FileResolver{
			Paths:      []string{p},
			JSONPath:   "token",
			ExtraPaths: map[string]string{"account_id": "account_id"},
		}
		cred, err := r.Resolve(context.Background())
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil", err)
		}
		if cred.Extra["account_id"] != canary {
			t.Fatalf("Resolve() Extra = %v, want canary present", cred.Extra)
		}
		if strings.Contains(cred.String(), canary) {
			t.Fatalf("Credential.String() = %q leaks canary", cred.String())
		}
		for _, w := range r.Warnings() {
			if strings.Contains(w, canary) {
				t.Fatalf("warning %q leaks canary", w)
			}
		}
		if len(r.Warnings()) != 1 {
			t.Fatalf("Warnings() = %v, want exactly 1", r.Warnings())
		}
	})
}
