package service

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeHarnessFixture(t *testing.T, home, path, body string) {
	t.Helper()
	dest := filepath.Join(home, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestDiscoverHarnessProviders(t *testing.T) {
	cases := []struct {
		slug, path, body string
		want             []string
	}{
		{"opencode", ".config/opencode/opencode.jsonc", `{"provider":{"anthropic":{},"github-copilot":{},"openrouter":{}}, // comment
   "disabled_providers":["anthropic"],"model":"openrouter/a/model",}`, []string{"copilot", "openrouter"}},
		{"opencode", ".local/share/opencode/auth.json", `{"openai":{"type":"oauth","access":"test-only"},"anthropic":{"type":"api","key":"test-only"}}`, []string{"claude", "codex"}},
		{"kilo", ".config/kilo/kilo.json", `{"provider":{"openai":{},"ollama":{}},"enabled_providers":["ollama"]}`, []string{"ollama"}},
		{"cline", ".cline/data/settings/providers.json", `{"version":1,"providers":{"anthropic":{"settings":{"provider":"anthropic","apiKey":"test-only"}},"github-copilot":{}}}`, []string{"claude", "copilot"}},
		{"cline", ".cline/data/globalState.json", `{"actModeApiProvider":"openrouter","planModeApiProvider":"anthropic"}`, []string{"claude", "openrouter"}},
		{"codex", ".codex/config.toml", "model_provider = \"local\"\n[model_providers.local]\nbase_url = \"http://localhost:1234/v1\"", []string{"local"}},
		{"claude", ".claude/settings.json", `{"env":{"CLAUDE_CODE_USE_BEDROCK":"1"}}`, []string{"amazon-bedrock"}},
		{"gemini", ".gemini/settings.json", `{"security":{"auth":{"selectedType":"oauth-personal"}}}`, []string{"google"}},
		{"qwen", ".qwen/settings.json", `{"modelProviders":{"anthropic":[],"gemini":[]}}`, []string{"claude", "google"}},
		{"goose", ".config/goose/config.yaml", "GOOSE_PROVIDER: openrouter\nGOOSE_MODEL: example", []string{"openrouter"}},
		{"aider", ".aider.conf.yml", "model: anthropic/claude-sonnet\nweak-model: openai/gpt-model", []string{"claude", "codex"}},
		{"continue", ".continue/config.yaml", "models:\n  - name: Local\n    provider: ollama\n    model: coder\n  - name: Cloud\n    provider: anthropic\n    model: sonnet", []string{"claude", "ollama"}},
		{"crush", ".config/crush/crush.json", `{"providers":{"anthropic":{},"openai":{"disable":true}}}`, []string{"claude"}},
		{"droid", ".factory/settings.json", `{"customModels":[{"provider":"anthropic","model":"example"}]}`, []string{"claude", "factory"}},
		{"opencode", ".config/opencode/opencode.json", `{"provider":{"$(touch bad)":{},"openrouter":{"options":{"baseURL":"https://example.test/a//b/*c*/"}}}}`, []string{"openrouter"}},
	}
	for _, tc := range cases {
		t.Run(tc.slug+"/"+tc.path, func(t *testing.T) {
			home := t.TempDir()
			writeHarnessFixture(t, home, tc.path, tc.body)
			got := discoverHarnessProviders(home, tc.slug)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			after, _ := os.ReadFile(filepath.Join(home, tc.path))
			if string(after) != tc.body {
				t.Fatal("discovery changed source config")
			}
		})
	}
}
func TestHarnessDiscoveryDegradesWithoutLeakingConfig(t *testing.T) {
	for _, body := range []string{`{"provider":`, strings.Repeat("x", harnessConfigLimit+1)} {
		home := t.TempDir()
		writeHarnessFixture(t, home, ".config/opencode/opencode.json", body)
		if got := discoverHarnessProviders(home, "opencode"); len(got) != 0 {
			t.Fatalf("malformed/oversized input produced providers: %v", got)
		}
	}
}
func TestHarnessDiscoveryPreservesExplicitSwitches(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(providersFixture))
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	path := ".config/opencode/opencode.json"
	writeHarnessFixture(t, svc.harnessHome, path, `{"provider":{"anthropic":{}}}`)
	find := func() HarnessInfo {
		for _, h := range mustListHarnesses(t, svc) {
			if h.Slug == "opencode" {
				return h
			}
		}
		t.Fatal("opencode missing")
		return HarnessInfo{}
	}
	if !find().Providers["claude"] {
		t.Fatal("configured provider not detected")
	}
	if err := svc.Harnesses().SetProvider(context.Background(), "opencode", "claude", false); err != nil {
		t.Fatal(err)
	}
	writeHarnessFixture(t, svc.harnessHome, path, `{"provider":{"anthropic":{},"openai":{}}}`)
	h := find()
	if h.Providers["claude"] || !h.Providers["codex"] {
		t.Fatalf("override/new discovery wrong: %v", h.Providers)
	}
	if err := svc.Harnesses().SetEnabled(context.Background(), "opencode", false); err != nil {
		t.Fatal(err)
	}
	if find().Providers["claude"] {
		t.Fatal("enable toggle lost provider override")
	}
	if err := svc.Harnesses().SetAllProviders(context.Background(), "opencode", false); err != nil {
		t.Fatal(err)
	}
	for _, on := range find().Providers {
		if on {
			t.Fatal("bulk disable was overwritten")
		}
	}
	t.Setenv("PATH", t.TempDir())
	if find().Installed {
		t.Fatal("uninstalled binary marked installed")
	}
}
func TestHarnessLegacyMigrationPreservesOverrides(t *testing.T) {
	body := providersFixture + `[harnesses.cursor]
name="Cursor"
command="cursor --model {model_id}"
builtin=true
providers=["cursor"]
enabled=false
[harnesses.cursor.provider_overrides]
cursor=false
`
	svc, _ := newTestServices(t, WithConfigTOML(body))
	mustListHarnesses(t, svc)
	hs, err := svc.cfg.LoadHarnesses()
	if err != nil {
		t.Fatal(err)
	}
	h := hs["cursor"]
	if h.Command != "cursor-agent --model {model_id}" || h.Enabled == nil || *h.Enabled || h.ProviderOverrides["cursor"] {
		t.Fatalf("migration lost settings: %+v", h)
	}
}
func TestHarnessQualifiedLaunchCommands(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[gui]\ncopy_command_instead=true\n"))
	mustListHarnesses(t, svc)
	for _, tc := range []struct{ slug, route, want string }{{"opencode", "claude/sonnet@high", "opencode --model anthropic/sonnet"}, {"kilo", "copilot/gpt-model@high", "kilo --model github-copilot/gpt-model"}, {"opencode", "zai-coding-plan/glm-5@high", "opencode --model zai-coding-plan/glm-5"}, {"cline", "claude/sonnet@high", "cline --model sonnet --provider anthropic"}} {
		r, err := svc.Harnesses().Launch(context.Background(), tc.slug, tc.route, "default")
		if err != nil || r.Command != tc.want {
			t.Fatalf("%s got %q err=%v want %q", tc.slug, r.Command, err, tc.want)
		}
	}
}

func TestClineLaunchRetainsOAuthAdapter(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[gui]\ncopy_command_instead=true\n"))
	mustListHarnesses(t, svc)
	writeHarnessFixture(t, svc.harnessHome, ".cline/data/settings/providers.json", `{"lastUsedProvider":"openai-codex","providers":{"openai-codex":{}}}`)
	result, err := svc.Harnesses().Launch(context.Background(), "cline", "codex/gpt-model@high", "research")
	if err != nil || result.Command != "cline --model gpt-model --provider openai-codex" {
		t.Fatalf("got %q err=%v", result.Command, err)
	}
}

func TestHarnessGatewayOutsideGlobalCatalogRemainsVisible(t *testing.T) {
	svc, _ := newTestServices(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "cline"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	writeHarnessFixture(t, svc.harnessHome, ".cline/data/settings/providers.json", `{"providers":{"cline":{}}}`)
	find := func() HarnessInfo {
		for _, h := range mustListHarnesses(t, svc) {
			if h.Slug == "cline" {
				return h
			}
		}
		t.Fatal("missing Cline")
		return HarnessInfo{}
	}
	if !find().Providers["cline"] {
		t.Fatal("gateway omitted")
	}
	if err := svc.Harnesses().SetProvider(context.Background(), "cline", "cline", false); err != nil {
		t.Fatal(err)
	}
	if find().Providers["cline"] {
		t.Fatal("gateway override lost")
	}
	if _, exists := svc.cfg.Providers["cline"]; exists {
		t.Fatal("discovery enabled a global provider")
	}
}
