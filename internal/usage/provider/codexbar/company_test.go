//go:build !nousage

package codexbar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func companyFixture(t *testing.T) *company.Policy {
	t.Helper()
	p := company.Defaults()
	p.AllowedProviders = []string{"claude", "antigravity"}
	dir := t.TempDir()
	p.CodexBarInstallations = []company.CodexBarInstallation{{Path: filepath.Join(dir, "approved.exe"), SHA256: strings.Repeat("a", 64), Config: company.Installation{Path: filepath.Join(dir, "approved.json"), SHA256: strings.Repeat("b", 64)}}}
	oldPolicy, oldVerify, oldRun, oldLook := readCompanyPolicy, verifyCompanyInstallation, runCompanyCommand, lookPath
	t.Cleanup(func() {
		readCompanyPolicy, verifyCompanyInstallation, runCompanyCommand, lookPath = oldPolicy, oldVerify, oldRun, oldLook
	})
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	verifyCompanyInstallation = func(company.Installation, bool) error { return nil }
	lookPath = func(string) (string, error) { t.Fatal("company attempted PATH discovery"); return "", nil }
	runCompanyCommand = func(*exec.Cmd) error { t.Fatal("unexpected delegated process"); return nil }
	return &p
}
func companyPayload(provider, source string) string {
	data, _ := json.Marshal([]map[string]any{{"provider": provider, "source": source, "usage": map[string]any{"updatedAt": time.Now().UTC().Format(time.RFC3339), "primary": map[string]any{"usedPercent": 25}}}})
	return string(data)
}
func TestCompanyCodexBarDefaultDeniesDiscoveryAndDelegation(t *testing.T) {
	p := companyFixture(t)
	p.CodexBarInstallations = nil
	if ids := SupportedProviders(); len(ids) != 0 {
		t.Fatal(ids)
	}
	if Preflight("claude") == nil {
		t.Fatal("preflight accepted no approval")
	}
	if _, err := FetchWithSourceEnvironment(context.Background(), "claude", "", map[string]string{"SECRET_CANARY": "value"}); err == nil {
		t.Fatal("unapproved delegation accepted")
	}
}
func TestCompanyCodexBarVerifiesImageAndConfigBeforeRun(t *testing.T) {
	for _, failImage := range []bool{true, false} {
		t.Run(map[bool]string{true: "image", false: "config"}[failImage], func(t *testing.T) {
			companyFixture(t)
			verifyCompanyInstallation = func(_ company.Installation, image bool) error {
				if image == failImage {
					return errors.New("SECRET_PATH_CANARY")
				}
				return nil
			}
			if err := Preflight("claude"); err == nil || strings.Contains(err.Error(), "CANARY") {
				t.Fatal(err)
			}
			if _, err := Fetch(context.Background(), "claude"); err == nil || strings.Contains(err.Error(), "CANARY") {
				t.Fatal(err)
			}
		})
	}
}
func TestCompanyCodexBarApprovedEnvironmentAndArguments(t *testing.T) {
	p := companyFixture(t)
	t.Setenv("SECRET_CANARY", "secret")
	t.Setenv("CODEXBAR_CONFIG", "UNAPPROVED_CANARY")
	t.Setenv("CODEXBAR_BIN", "UNAPPROVED_CANARY")
	t.Setenv("HTTPS_PROXY", "SECRET_CANARY")
	verified := 0
	verifyCompanyInstallation = func(company.Installation, bool) error { verified++; return nil }
	runCompanyCommand = func(cmd *exec.Cmd) error {
		if verified != 2 || cmd.Path != p.CodexBarInstallations[0].Path || cmd.Dir != filepath.Dir(cmd.Path) {
			t.Fatalf("unverified image: %+v", cmd)
		}
		want := []string{cmd.Path, "usage", "--provider", "claude", "--format", "json", "--json-only", "--no-color", "--source", "web"}
		if !reflect.DeepEqual(cmd.Args, want) {
			t.Fatal(cmd.Args)
		}
		joined := strings.Join(cmd.Env, "\n")
		if strings.Contains(joined, "CANARY") || !strings.Contains(joined, "CODEXBAR_CONFIG="+p.CodexBarInstallations[0].Config.Path) {
			t.Fatal("parent environment escaped or config missing")
		}
		if cmd.Stderr != io.Discard || cmd.WaitDelay != time.Second {
			t.Fatal("unbounded output contract")
		}
		_, err := io.WriteString(cmd.Stdout, companyPayload("claude", "web"))
		return err
	}
	snap, err := FetchWithSource(context.Background(), "claude", usage.SourceWeb)
	if err != nil || snap.Failure != nil || !snap.UsageKnown {
		t.Fatalf("approved: %+v %v", snap, err)
	}
}
func TestCompanyCodexBarDelegatesOnlyAntigravityCredential(t *testing.T) {
	companyFixture(t)
	token := `{"access_token":"SYNTHETIC_TOKEN","expiry_date":0,"client_id":"SYNTHETIC_CLIENT","client_secret":"SYNTHETIC_CLIENT_SECRET"}`
	runCompanyCommand = func(cmd *exec.Cmd) error {
		found := false
		for _, entry := range cmd.Env {
			if entry == antigravityCredentialKey+"="+token {
				found = true
			}
		}
		if !found || strings.Contains(strings.Join(cmd.Args, " "), "SYNTHETIC_TOKEN") {
			t.Fatal("credential input contract")
		}
		_, err := io.WriteString(cmd.Stdout, companyPayload("antigravity", "oauth"))
		return err
	}
	snap, err := FetchWithSourceEnvironment(context.Background(), "antigravity", usage.SourceOAuth, map[string]string{antigravityCredentialKey: strings.TrimSuffix(token, "}") + `,"unrelated":"SECRET_CANARY"}`})
	if err != nil || snap.Failure != nil {
		t.Fatalf("delegation: %+v %v", snap, err)
	}
	runCompanyCommand = func(*exec.Cmd) error { t.Fatal("unsupported credentials reached process"); return nil }
	for _, provider := range []string{"claude", "antigravity"} {
		if _, err := FetchWithSourceEnvironment(context.Background(), provider, usage.SourceWeb, map[string]string{antigravityCredentialKey: token}); err == nil {
			t.Fatal("unapproved credential transport")
		}
	}
	if _, err := FetchWithSourceEnvironment(context.Background(), "antigravity", usage.SourceOAuth, map[string]string{"GITHUB_TOKEN": "SECRET_CANARY"}); err == nil || strings.Contains(err.Error(), "CANARY") {
		t.Fatal(err)
	}
}
func TestCompanyCodexBarOutputAndTimeoutMatrix(t *testing.T) {
	for _, scenario := range []string{"web-alias", "wrong-provider", "duplicate", "wrong-source", "unknown-source", "invalid-time", "bad-exit", "provider-error", "oversized", "invalid-json", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			companyFixture(t)
			budget := 5 * time.Second
			if scenario == "timeout" {
				budget = 20 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			runCompanyCommand = func(cmd *exec.Cmd) error {
				data := companyPayload("claude", "web")
				switch scenario {
				case "web-alias":
					data = companyPayload("claude", "openai-web")
				case "wrong-provider":
					data = companyPayload("codex", "web")
				case "duplicate":
					data = strings.TrimSuffix(data, "]") + "," + strings.TrimPrefix(data, "[")
				case "wrong-source":
					data = companyPayload("claude", "oauth")
				case "unknown-source":
					data = companyPayload("claude", "unknown")
				case "invalid-time":
					data = `[{"provider":"claude","source":"web","usage":{"updatedAt":"bad","primary":{"usedPercent":25}}}]`
				case "provider-error":
					data = `[{"provider":"claude","error":{"code":"unauthorized","message":"SECRET_CANARY"}}]`
				case "oversized":
					data = strings.Repeat("x", maxStdoutBytes+1)
				case "invalid-json":
					data = "SECRET_CANARY"
				case "timeout":
					<-ctx.Done()
					return ctx.Err()
				}
				io.WriteString(cmd.Stdout, data)
				if scenario == "bad-exit" {
					return errors.New("SECRET_CANARY")
				}
				return nil
			}
			snap, err := FetchWithSource(ctx, "claude", usage.SourceWeb)
			if scenario == "web-alias" {
				if err != nil || snap.Failure != nil || snap.Source != usage.SourceWeb {
					t.Fatalf("web source alias lost: %+v %v", snap, err)
				}
				return
			}
			if scenario == "invalid-time" {
				if err != nil || !snap.Stale || !snap.FetchedAt.IsZero() {
					t.Fatalf("invalid timestamp appears fresh: %+v %v", snap, err)
				}
				return
			}
			if err != nil || snap.Failure == nil || strings.Contains(snap.Failure.Message, "CANARY") {
				t.Fatalf("unsafe result: %+v %v", snap, err)
			}
		})
	}
}
