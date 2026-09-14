package service

import (
	"context"
	"errors"
	"github.com/WD-Mitchell/which-model/internal/approvedexec"
	"github.com/WD-Mitchell/which-model/internal/company"
	"os/exec"
	"strings"
	"testing"
)

func TestCompanyApprovedLaunchUsesProtectedArguments(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[harnesses.codex]\nname='changed'\ncommand='PROJECT_SHELL_CANARY'\nbuiltin=false\n"))
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	p.Executables = []company.Executable{{ID: "codex", Path: "/approved/image", Args: []string{"-m", "{model_id}", "-c", "model_reasoning_effort=high"}}}
	oldPolicy, oldVerify, oldStart := readCompanyPolicy, verifyCompanyPlan, startCompanyProcess
	t.Cleanup(func() { readCompanyPolicy, verifyCompanyPlan, startCompanyProcess = oldPolicy, oldVerify, oldStart })
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	verified, started := false, false
	verifyCompanyPlan = func(plan approvedexec.Plan) error {
		verified = true
		if plan.Path != "/approved/image" {
			t.Fatal("wrong image")
		}
		return nil
	}
	startCompanyProcess = func(cmd *exec.Cmd) error {
		started = true
		if !verified || cmd.Path != "/approved/image" || strings.Join(cmd.Args[1:], "|") != "-m|gpt-test|-c|model_reasoning_effort=high" {
			t.Fatalf("wrong process: %+v", cmd)
		}
		if cmd.Stdout != nil || cmd.Stderr != nil || len(cmd.Env) == 0 {
			t.Fatal("unsafe capture/environment")
		}
		for _, v := range cmd.Env {
			if strings.Contains(v, "CANARY") {
				t.Fatal("environment leaked")
			}
		}
		return nil
	}
	t.Setenv("PATH", "SUBSTITUTE_PATH_CANARY")
	t.Setenv("OPENAI_API_KEY", "CREDENTIAL_CANARY")
	if _, err := svc.Harnesses().Launch(context.Background(), "codex", "codex/gpt-test@high", "balanced"); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("approved process did not launch")
	}
	started = false
	verifyCompanyPlan = func(approvedexec.Plan) error { return errors.New("changed image") }
	if _, err := svc.Harnesses().Launch(context.Background(), "codex", "codex/gpt-test@high", "balanced"); err == nil || started {
		t.Fatal("changed installation launched")
	}
}
func TestCompanyCustomShellCannotBeAuthorizedByConfiguration(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[harnesses.custom]\nname='custom'\ncommand='PROJECT_SHELL_CANARY'\nbuiltin=true\n"))
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	p.Executables = []company.Executable{{ID: "custom", Path: "/approved/shell"}}
	oldPolicy, oldStart := readCompanyPolicy, startCompanyProcess
	t.Cleanup(func() { readCompanyPolicy, startCompanyProcess = oldPolicy, oldStart })
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	startCompanyProcess = func(*exec.Cmd) error { t.Fatal("unapproved custom started"); return nil }
	if _, err := svc.Harnesses().Launch(context.Background(), "custom", "codex/model@high", "balanced"); err == nil {
		t.Fatal("mutable builtin flag authorized custom shell")
	}
}
