package service

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/hooks"
	"github.com/WD-Mitchell/which-model/internal/skills"
)

type approvedChildResult struct {
	Executable string
	Args, Env  []string
}

func TestApprovedExecutionChildHelper(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) != 5 {
		t.Fatal("invalid fixture args")
	}
	image, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(approvedChildResult{Executable: image, Args: args[1:], Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(args[0], data, 0600); err != nil {
		t.Fatal(err)
	}
}
func nativeExecutionPolicy(t *testing.T) company.Snapshot {
	t.Helper()
	if os.Getenv("COMPANY_EXECUTION_TEST_DIR") == "" {
		t.Skip("isolated native CI fixture required")
	}
	policy, err := company.Load()
	if err != nil || !policy.Managed || len(policy.Policy.Executables) != 1 {
		t.Fatalf("fixture policy unavailable: %v", err)
	}
	return policy
}
func TestNativeCompanyExecution(t *testing.T) {
	policy := nativeExecutionPolicy(t)
	entry := policy.Policy.Executables[0]
	if err := company.VerifyInstallation(company.Installation{Path: entry.Path, SHA256: entry.SHA256}, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"changed.exe", "writable.exe", "redirect.exe"} {
		if err := company.VerifyInstallation(company.Installation{Path: filepath.Join(os.Getenv("COMPANY_EXECUTION_TEST_DIR"), name), SHA256: entry.SHA256}, true); err == nil {
			t.Fatalf("unsafe native installation accepted: %s", name)
		}
	}
	output := os.Getenv("COMPANY_EXECUTION_OUTPUT")
	defer os.Remove(output)
	dir := t.TempDir()
	svc := NewEmpty(config.Paths{StateDir: dir}, config.Default(), nil)
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "NODE_OPTIONS", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "HTTPS_PROXY", "SHELL", "LD_PRELOAD", "DYLD_INSERT_LIBRARIES", "SystemRoot", "LOCALAPPDATA", "APPDATA"} {
		t.Setenv(key, "NATIVE_PARENT_CANARY")
	}
	if _, err := svc.Harnesses().Launch(context.Background(), "codex", "codex/gpt-test@high", "balanced"); err != nil {
		t.Fatal(err)
	}
	var result approvedChildResult
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(output)
		if err == nil && json.Unmarshal(data, &result) == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !reflect.DeepEqual(result.Args, []string{"gpt-test", "high", "codex", "balanced"}) {
		t.Fatalf("native argument boundary failure: %+v", result.Args)
	}
	want, err := os.Stat(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.Stat(result.Executable)
	if err != nil || !os.SameFile(want, got) {
		t.Fatal("unapproved executable ran")
	}
	for _, value := range result.Env {
		if strings.Contains(value, "CANARY") {
			t.Fatal("native child inherited parent canary")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "launch.log")); !os.IsNotExist(err) {
		t.Fatal("raw launch log created")
	}
	if _, err := os.Stat(filepath.Join(dir, "launch.jsonl")); err != nil {
		t.Fatal("structured native launch missing")
	}
}
func TestNativeCompanyIntegrationInstall(t *testing.T) {
	nativeExecutionPolicy(t)
	root := os.Getenv("COMPANY_EXECUTION_PROJECT")
	if err := os.MkdirAll(filepath.Join(root, "skills", "model-selection", "agents"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"SKILL.md", "agents/openai.yaml"} {
		if err := os.WriteFile(filepath.Join(root, "skills", "model-selection", filepath.FromSlash(rel)), []byte("synthetic owned integration\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	skills.SetRepoDir(root)
	defer skills.SetRepoDir("")
	if _, err := skills.Install("model-selection", skills.TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(foreign), 0700)
	os.WriteFile(foreign, []byte(`{"foreign_setting":"FOREIGN_CANARY","hooks":{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"foreign-command"}]}]}}`), 0600)
	if _, err := hooks.Install("claude", hooks.Installed(hooks.VariantUsage), root); err != nil {
		t.Fatal(err)
	}
	ran := false
	_, err := hooks.Run("usage-refresh", nil, hooks.Options{Runner: func([]string, io.Writer, io.Writer) int { ran = true; return 0 }})
	if err != nil || !ran {
		t.Fatalf("approved hook use did not reach runner: %v", err)
	}
}
func TestNativeCompanyIntegrationRemoval(t *testing.T) {
	policy := nativeExecutionPolicy(t)
	if policy.Policy.Integrations.HookInstallation || policy.Policy.Integrations.SkillInstallation || policy.Policy.Integrations.HookUse {
		t.Fatal("installation was not disabled")
	}
	root := os.Getenv("COMPANY_EXECUTION_PROJECT")
	skills.SetRepoDir(root)
	defer skills.SetRepoDir("")
	if _, err := skills.Install("model-selection", skills.TargetGeneric, false, true); err == nil {
		t.Fatal("disabled skill install allowed")
	}
	if _, err := hooks.Install("claude", hooks.Installed(hooks.VariantUsage), root); err == nil {
		t.Fatal("disabled hook install allowed")
	}
	if _, err := skills.Remove("model-selection", skills.TargetGeneric, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := hooks.Remove("claude", root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "model-selection", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("owned skill was not removed")
	}
	data, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil || !strings.Contains(string(data), "FOREIGN_CANARY") || !strings.Contains(string(data), "foreign-command") || strings.Contains(string(data), "which-model hooks run") {
		t.Fatal("foreign/owned hook removal mismatch")
	}
	ran := false
	_, err = hooks.Run("usage-refresh", nil, hooks.Options{Runner: func([]string, io.Writer, io.Writer) int { ran = true; return 0 }})
	if err == nil || ran {
		t.Fatal("disabled hook ran")
	}
}
