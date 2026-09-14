package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/company"
)

func withCompanyPolicy(t *testing.T) {
	t.Helper()
	old := readCompanyPolicy
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	t.Cleanup(func() { readCompanyPolicy = old })
}

func TestManagedConfigurationPrecedence(t *testing.T) {
	withCompanyPolicy(t)
	for _, source := range []string{"user", "project", "explicit flag", "explicit environment", "key environment"} {
		t.Run(source, func(t *testing.T) {
			home, project := t.TempDir(), t.TempDir()
			explicit := ""
			env := map[string]string{}
			path := filepath.Join(home, ".config", "which-model", "config.toml")
			switch source {
			case "project":
				path = filepath.Join(project, ".which-model", "config.toml")
			case "explicit flag":
				path = filepath.Join(t.TempDir(), "alternate.toml")
				explicit = path
			case "explicit environment":
				path = filepath.Join(t.TempDir(), "alternate.toml")
				env["WHICH_MODEL_CONFIG"] = path
			case "key environment":
				t.Setenv("WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED", "true")
				env["WHICH_MODEL_PROVIDERS_CLAUDE_ENABLED"] = "true"
			}
			content := "[providers.claude]\nenabled=true\n"
			if source == "key environment" {
				content = ""
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(LoadOptions{Path: explicit, Home: home, CWD: project, GOOS: "linux", Getenv: func(k string) string { return env[k] }})
			var denied *company.Error
			if !errors.As(err, &denied) {
				t.Fatalf("%s override not refused by policy: %v", source, err)
			}
		})
	}
}

func TestManagedStorageAndDesktopMutation(t *testing.T) {
	withCompanyPolicy(t)
	cfg := Default()
	if err := cfg.SetAuth(AuthConfig{UseKeychain: false}); err == nil {
		t.Fatal("desktop storage override accepted")
	}
	auth, err := cfg.LoadAuth()
	if err != nil || !auth.UseKeychain {
		t.Fatal("rejected mutation changed state")
	}
	if err := ValidateManagedDocument([]byte("[auth]\nuse_keychain=false\n")); err == nil {
		t.Fatal("config set payload bypassed policy")
	}
	if err := ValidateManagedDocument([]byte("[company]\nmanaged=false\n")); err == nil {
		t.Fatal("user document supplied policy authority")
	}
}

func TestRequiredPolicyPrecedesOrdinaryConfigReads(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	readCompanyPolicy = func() (company.Snapshot, error) {
		return company.Snapshot{}, &company.Error{Reason: "required policy missing"}
	}
	_, err := Load(LoadOptions{Path: filepath.Join(t.TempDir(), "ordinary-CANARY.toml")})
	if err == nil || strings.Contains(err.Error(), "CANARY") {
		t.Fatal("ordinary config was read before required policy", err)
	}
}

func TestManagedRankingPreferencesRemainAvailable(t *testing.T) {
	withCompanyPolicy(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[strategy]\ndefault_profile='research'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(LoadOptions{Path: path, Getenv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	var strategy struct {
		DefaultProfile string `toml:"default_profile"`
	}
	if err := cfg.UnmarshalKey("strategy", &strategy); err != nil || strategy.DefaultProfile != "research" {
		t.Fatal("ranking preference lost", err)
	}
	data, err := cfg.Clone().MarshalTOML()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secure_store_only") || strings.Contains(string(data), "allowed_providers") {
		t.Fatal("administrator authority serialized into user config")
	}
}
