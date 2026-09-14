package approvedexec

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"strings"
	"testing"
)

func TestApprovedPlanIgnoresMutableCommandsAndPreservesArguments(t *testing.T) {
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	p.Executables = []company.Executable{{ID: "codex", Path: "/approved/codex", SHA256: strings.Repeat("a", 64), Args: []string{"-m", "{model_id}", "-c", "model_reasoning_effort=high"}}}
	s := company.Snapshot{Managed: true, Policy: &p}
	plan, err := Build(s, "codex", true, map[string]string{"model_id": "gpt-test", "provider": "codex", "reasoning": "high", "profile": "balanced"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Path != "/approved/codex" || strings.Join(plan.Args, "|") != "-m|gpt-test|-c|model_reasoning_effort=high" {
		t.Fatalf("wrong argv: %+v", plan)
	}
	p.Executables[0].Args = []string{"--model={model_id}"}
	if _, err := Build(s, "codex", true, map[string]string{"model_id": "safe"}); err == nil {
		t.Fatal("embedded placeholder accepted")
	}
}
func TestApprovedPlanRejectsUnapprovedCustomAndArgumentInjection(t *testing.T) {
	p := company.Defaults()
	p.Executables = []company.Executable{{ID: "custom", Path: "/approved/shell", Args: []string{"-c", "fixed script", "--", "{model_id}"}}}
	s := company.Snapshot{Managed: true, Policy: &p}
	if _, err := Build(s, "custom", false, map[string]string{"model_id": "model"}); err == nil {
		t.Fatal("custom permission bypass")
	}
	p.AllowCustomShell = true
	for _, value := range []string{"--injected", "model;payload", "$(payload)", "two words", "a\nline", "%TOKEN%", "a&b", "a|b"} {
		if _, err := Build(s, "custom", false, map[string]string{"model_id": value}); err == nil {
			t.Fatalf("unsafe route value accepted: %q", value)
		}
	}
	if _, err := Build(s, "custom", false, map[string]string{"model_id": "provider/model-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(s, "missing", true, nil); err == nil {
		t.Fatal("missing approval accepted")
	}
}
func TestApprovedEnvironmentExcludesInheritedSecrets(t *testing.T) {
	for _, key := range []string{"HOME", "USERPROFILE", "PATH", "NODE_OPTIONS", "PYTHONPATH", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "HTTPS_PROXY", "GIT_CONFIG_COUNT", "SHELL", "LD_PRELOAD", "DYLD_INSERT_LIBRARIES", "APPDATA", "LOCALAPPDATA", "SystemRoot", "TMPDIR"} {
		t.Setenv(key, "SYNTHETIC_ENV_CANARY")
	}
	env, err := Environment()
	if err != nil {
		t.Fatal(err)
	}
	if len(env) == 0 {
		t.Fatal("empty environment")
	}
	for _, item := range env {
		if strings.Contains(item, "CANARY") {
			t.Fatal("parent environment leaked")
		}
	}
}

func TestApprovedBuiltinCannotImplicitlySelectShell(t *testing.T) {
	p := company.Defaults()
	p.Executables = []company.Executable{{ID: "codex", Path: "/bin/sh", Args: []string{"-c", "fixed script"}}}
	s := company.Snapshot{Managed: true, Policy: &p}
	if _, err := Build(s, "codex", true, nil); err == nil {
		t.Fatal("shell image used without separate permission")
	}
	p.AllowCustomShell = true
	if _, err := Build(s, "codex", true, nil); err != nil {
		t.Fatal(err)
	}
}
