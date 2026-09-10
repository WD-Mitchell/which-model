//go:build !nousage

package codexbar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

type nativeCapture struct {
	Args, Environment []string
	Directory         string
}

// This helper exists only in the test executable. CI installs that native image
// under protected ownership; no production CLI bypass or credential is involved.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "usage" {
		var cfg struct {
			FixtureOutput string `json:"fixtureOutput"`
		}
		raw, err := os.ReadFile(os.Getenv("CODEXBAR_CONFIG"))
		if err != nil || json.Unmarshal(raw, &cfg) != nil || cfg.FixtureOutput == "" {
			os.Exit(9)
		}
		cwd, _ := os.Getwd()
		data, _ := json.Marshal(nativeCapture{Args: os.Args[1:], Environment: os.Environ(), Directory: cwd})
		if os.WriteFile(cfg.FixtureOutput, data, 0600) != nil {
			os.Exit(10)
		}
		os.Stdout.WriteString(companyPayload("antigravity", "oauth"))
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestNativeCompanyCodexBarApproval(t *testing.T) {
	dir := os.Getenv("COMPANY_CODEXBAR_TEST_DIR")
	output := os.Getenv("COMPANY_CODEXBAR_OUTPUT")
	if dir == "" || output == "" {
		t.Skip("disposable native CI fixture only")
	}
	policy, err := company.Load()
	if err != nil || !policy.Managed {
		t.Fatal("native fixture policy unavailable")
	}
	t.Setenv("CODEXBAR_BIN", "UNAPPROVED_CANARY")
	t.Setenv("CODEXBAR_CONFIG", "UNAPPROVED_CANARY")
	t.Setenv("UNRELATED_TOKEN_CANARY", "SECRET_CANARY")
	t.Setenv("HTTPS_PROXY", "SECRET_CANARY")
	if err := Preflight("antigravity"); err != nil {
		t.Fatal(err)
	}
	snap, err := FetchWithSourceEnvironment(context.Background(), "antigravity", usage.SourceOAuth, map[string]string{antigravityCredentialKey: `{"access_token":"SYNTHETIC_NATIVE_TOKEN","expiry_date":0,"client_id":"SYNTHETIC_CLIENT","client_secret":"SYNTHETIC_CLIENT_SECRET"}`})
	if err != nil || snap.Failure != nil || !snap.UsageKnown {
		t.Fatalf("native approved result: %v %v", err, snap.Failure)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var got nativeCapture
	if json.Unmarshal(data, &got) != nil {
		t.Fatal("invalid capture")
	}
	want := []string{"usage", "--provider", "antigravity", "--format", "json", "--json-only", "--no-color", "--source", "oauth"}
	if !reflect.DeepEqual(got.Args, want) || !strings.EqualFold(filepath.Clean(got.Directory), filepath.Clean(dir)) {
		t.Fatal("native arguments/directory mismatch")
	}
	joined := strings.Join(got.Environment, "\n")
	if strings.Contains(joined, "CANARY") || !strings.Contains(joined, antigravityCredentialKey+`={"access_token":"SYNTHETIC_NATIVE_TOKEN","expiry_date":0,"client_id":"SYNTHETIC_CLIENT","client_secret":"SYNTHETIC_CLIENT_SECRET"}`) {
		t.Fatal("native child inherited unrelated environment or lost selected credential")
	}
	os.Remove(output)
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	for _, scenario := range []string{"changed-image", "writable-image", "redirect-image", "changed-config", "writable-config", "redirect-config", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			next := *policy.Policy
			next.CodexBarInstallations = append([]company.CodexBarInstallation(nil), policy.Policy.CodexBarInstallations...)
			entry := &next.CodexBarInstallations[0]
			if scenario == "disabled" {
				next.CodexBarInstallations = nil
			} else if strings.HasSuffix(scenario, "image") {
				entry.Path = filepath.Join(dir, scenario+".exe")
			} else {
				entry.Config.Path = filepath.Join(dir, scenario+".json")
			}
			readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &next}, nil }
			if err := Preflight("antigravity"); err == nil {
				t.Fatal("changed native approval passed")
			}
			if _, err := Fetch(context.Background(), "antigravity"); err == nil {
				t.Fatal("changed native image/config executed")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("refused native operation launched a child")
			}
		})
	}
}
