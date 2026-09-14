// Package approvedexec prepares company-approved native executable invocations.
// It never reads mutable command strings or resolves an image through PATH.
package approvedexec

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"path/filepath"
	"regexp"
	"strings"
)

type Plan struct {
	Path   string
	Args   []string
	Inputs []company.Installation
	SHA256 string
}

var valuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/+-]{0,255}$`)
var placeholder = regexp.MustCompile(`\{[A-Za-z0-9_]+\}`)

func refusal(reason string) error { return &company.Error{Reason: reason} }
func ValidValue(value string) bool {
	return valuePattern.MatchString(value) && !strings.Contains(value, "..") && !strings.Contains(value, "://")
}

// Build consumes only a protected snapshot. builtin is determined by the compiled
// registry at the call site, never by the user-controlled configuration flag.
func Build(policy company.Snapshot, id string, builtin bool, values map[string]string) (Plan, error) {
	if !policy.Managed || policy.Policy == nil {
		return Plan{}, refusal("protected executable policy is required")
	}
	if !builtin && !policy.Policy.AllowCustomShell {
		return Plan{}, refusal("custom shell execution is prohibited")
	}
	for _, entry := range policy.Policy.Executables {
		if entry.ID != id {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(entry.Path)), ".exe")
		switch name {
		case "sh", "bash", "dash", "zsh", "ksh", "fish", "cmd", "powershell", "pwsh":
			if !policy.Policy.AllowCustomShell {
				return Plan{}, refusal("shell images require separate custom shell permission")
			}
		}
		plan := Plan{Path: entry.Path, SHA256: entry.SHA256, Inputs: append([]company.Installation(nil), entry.Inputs...)}
		for _, arg := range entry.Args {
			if token := placeholder.FindString(arg); token != "" {
				if token != arg {
					return Plan{}, refusal("executable placeholders must be whole arguments")
				}
				key := strings.TrimSuffix(strings.TrimPrefix(token, "{"), "}")
				switch key {
				case "model_id", "reasoning", "provider", "profile":
				default:
					return Plan{}, refusal("unknown executable argument placeholder")
				}
				value, ok := values[key]
				if !ok || !ValidValue(value) {
					return Plan{}, refusal("unsafe or missing executable argument value")
				}
				arg = value
			}
			plan.Args = append(plan.Args, arg)
		}
		return plan, nil
	}
	return Plan{}, refusal("harness has no administrator-approved executable")
}
func (p Plan) Verify() error {
	if err := company.VerifyInstallation(company.Installation{Path: p.Path, SHA256: p.SHA256}, true); err != nil {
		return err
	}
	for _, input := range p.Inputs {
		if err := company.VerifyInstallation(input, false); err != nil {
			return err
		}
	}
	return nil
}
