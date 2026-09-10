package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/approvedexec"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

var verifyCompanyPlan = func(plan approvedexec.Plan) error { return plan.Verify() }
var startCompanyProcess = func(cmd *exec.Cmd) error { return cmd.Start() }

func compiledHarness(slug string) bool {
	for _, seed := range harnessSeeds {
		if seed.slug == slug {
			return true
		}
	}
	return false
}
func approvedHarness(policy company.Snapshot, slug string) (company.Executable, bool) {
	if policy.Policy != nil {
		for _, entry := range policy.Policy.Executables {
			if entry.ID == slug {
				return entry, true
			}
		}
	}
	return company.Executable{}, false
}
func displayApprovedCommand(plan approvedexec.Plan) string {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	prefix := ""
	if runtime.GOOS == "windows" {
		quote = func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
		prefix = "& "
	}
	words := []string{quote(plan.Path)}
	for _, arg := range plan.Args {
		words = append(words, quote(arg))
	}
	return prefix + strings.Join(words, " ")
}

func (h *HarnessService) launchCompany(ctx context.Context, policy company.Snapshot, slug, routeKey, profile string) (LaunchResult, error) {
	// Resolve authority before parsing untrusted display values or reading harness
	// config. Disabled company launches have no file/process effects.
	entry, approved := approvedHarness(policy, slug)
	if !approved {
		return LaunchResult{}, &company.Error{Reason: "harness has no administrator-approved executable"}
	}
	builtin := compiledHarness(slug)
	if !builtin && !policy.Policy.AllowCustomShell {
		return LaunchResult{}, &company.Error{Reason: "custom shell execution is prohibited"}
	}
	provider, model, reasoning, err := ParseRouteKey(routeKey)
	if err != nil {
		return LaunchResult{}, &company.Error{Reason: "invalid harness route"}
	}
	if err := policy.RequireProvider(provider); err != nil {
		return LaunchResult{}, err
	}
	for _, value := range []string{provider, model, reasoning, profile} {
		if !approvedexec.ValidValue(value) {
			return LaunchResult{}, &company.Error{Reason: "unsafe harness route or profile value"}
		}
	}
	env, err := approvedexec.Environment()
	if err != nil {
		return LaunchResult{}, err
	}
	mappedProvider := provider
	if builtin && (slug == "opencode" || slug == "kilo") {
		model = routing.CatalogueSlugFor(provider) + "/" + model
	}
	if builtin && slug == "cline" {
		mappedProvider = routing.CatalogueSlugFor(provider)
		home := ""
		for _, value := range env {
			if strings.HasPrefix(value, "HOME=") {
				home = strings.TrimPrefix(value, "HOME=")
			}
		}
		if home != "" && policy.RequireSource("provider_file") == nil {
			if configured := clineProviderID(home, provider); configured != "" {
				mappedProvider = configured
			}
		}
	}
	plan, err := approvedexec.Build(policy, entry.ID, builtin, map[string]string{"provider": mappedProvider, "model_id": model, "reasoning": reasoning, "profile": profile})
	if err != nil {
		return LaunchResult{}, err
	}
	if err := verifyCompanyPlan(plan); err != nil {
		return LaunchResult{}, err
	}
	h.s.mu.RLock()
	gui, gerr := h.s.cfg.LoadGUI()
	h.s.mu.RUnlock()
	if gerr != nil {
		return LaunchResult{}, gerr
	}
	display := displayApprovedCommand(plan)
	if gui.CopyCommandInstead {
		h.recordCompanyLaunch(slug, provider, model, profile, "copied")
		h.recordPick(ctx, profile, routeKey)
		return LaunchResult{Copied: true, Command: display}, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return LaunchResult{}, ctx.Err()
	}
	proc := exec.Command(plan.Path, plan.Args...)
	proc.Env = env
	proc.SysProcAttr = launchSysProcAttr()
	// Nil stdio goes to the null device. No environment or command text is logged.
	if err := startCompanyProcess(proc); err != nil {
		h.recordCompanyLaunch(slug, provider, model, profile, "failed")
		return LaunchResult{}, fmt.Errorf("%w: approved harness could not be started", errLaunchFailed)
	}
	if proc.Process != nil {
		_ = proc.Process.Release()
	}
	h.recordCompanyLaunch(slug, provider, model, profile, "started")
	h.recordPick(ctx, profile, routeKey)
	return LaunchResult{Command: display}, nil
}
