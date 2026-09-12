//go:build !nousage

package codexbar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/approvedexec"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/antigravity"
)

var readCompanyPolicy = company.Load
var verifyCompanyInstallation = company.VerifyInstallation
var runCompanyCommand = func(cmd *exec.Cmd) error { return cmd.Run() }

const antigravityCredentialKey = "ANTIGRAVITY_OAUTH_CREDENTIALS_JSON"

func approvalError(reason string) error { return &company.Error{Reason: reason} }

func companyProviderIDs(policy company.Snapshot) []string {
	if policy.RequireCodexBar() != nil {
		return nil
	}
	ids := []string{}
	for _, id := range policy.Policy.AllowedProviders {
		if validCompanyProvider(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
func validCompanyProvider(id string) bool {
	return approvedexec.ValidValue(id) && id != "all" && id != "both" && !strings.ContainsAny(id, "/:")
}
func approvedInstallation(policy company.Snapshot) (company.CodexBarInstallation, error) {
	if err := policy.RequireCodexBar(); err != nil {
		return company.CodexBarInstallation{}, err
	}
	for _, entry := range policy.Policy.CodexBarInstallations {
		if verifyCompanyInstallation(company.Installation{Path: entry.Path, SHA256: entry.SHA256}, true) != nil {
			continue
		}
		if verifyCompanyInstallation(entry.Config, false) != nil {
			continue
		}
		return entry, nil
	}
	return company.CodexBarInstallation{}, approvalError("no approved CodexBar image and configuration passed verification")
}

// Preflight refuses an unapproved delegated operation before credential resolution.
// Personal operation does not perform discovery or read an executable here.
func Preflight(provider string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if !policy.Managed {
		return nil
	}
	if !validCompanyProvider(provider) {
		return approvalError("CodexBar requires one allowed provider")
	}
	if err := policy.RequireProvider(provider); err != nil {
		return err
	}
	_, err = approvedInstallation(policy)
	return err
}

func approvedEnvironment(entry company.CodexBarInstallation, provider string, source usage.Source, credentials map[string]string) ([]string, error) {
	env, err := approvedexec.Environment()
	if err != nil {
		return nil, err
	}
	env = append(env, "CODEXBAR_CONFIG="+entry.Config.Path)
	for key, value := range credentials {
		if key != antigravityCredentialKey || provider != "antigravity" || (source != "" && source != usage.SourceOAuth) || len(value) == 0 || len(value) > 64<<10 || strings.ContainsRune(value, 0) || !json.Valid([]byte(value)) || !strings.HasPrefix(strings.TrimSpace(value), "{") {
			return nil, approvalError("unsupported CodexBar credential environment")
		}
		var selected antigravity.Credentials
		if json.Unmarshal([]byte(value), &selected) != nil || selected.AccessToken == "" || selected.ClientID == "" || selected.ClientSecret == "" {
			return nil, approvalError("invalid CodexBar OAuth credential input")
		}
		// Keep the canonical auth/account-routing fields, never arbitrary stored JSON.
		encoded, err := json.Marshal(selected)
		if err != nil {
			return nil, approvalError("invalid CodexBar OAuth credential input")
		}
		env = append(env, key+"="+string(encoded))
	}
	return env, nil
}

func fetchApproved(ctx context.Context, policy company.Snapshot, provider string, source usage.Source, credentials map[string]string) (usage.Snapshot, error) {
	if !validCompanyProvider(provider) {
		return usage.Snapshot{}, approvalError("CodexBar requires one allowed provider")
	}
	if err := policy.RequireProvider(provider); err != nil {
		return usage.Snapshot{}, err
	}
	switch source {
	case "", usage.SourceAPI, usage.SourceOAuth, usage.SourceWeb, usage.SourceCLI:
	default:
		return usage.Snapshot{}, approvalError("unsupported live CodexBar source")
	}
	entry, err := approvedInstallation(policy)
	if err != nil {
		return usage.Snapshot{}, err
	}
	env, err := approvedEnvironment(entry, provider, source, credentials)
	if err != nil {
		return usage.Snapshot{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	args := []string{"usage", "--provider", provider, "--format", "json", "--json-only", "--no-color"}
	if source != "" {
		args = append(args, "--source", string(source))
	}
	cmd := exec.CommandContext(cmdCtx, entry.Path, args...)
	cmd.Env = env
	cmd.Dir = filepath.Dir(entry.Path)
	cmd.WaitDelay = time.Second
	var stdout cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	runErr := runCompanyCommand(cmd)
	failure := func(code, message string) (usage.Snapshot, error) {
		return failedSnapshot(provider, usage.SourceCLI, code, message), nil
	}
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		return failure("timeout", "codexbar usage request timed out")
	}
	if cmdCtx.Err() != nil {
		return failure("provider_status", "codexbar usage request was cancelled")
	}
	if stdout.tooLarge {
		return failure("response_too_large", "codexbar output exceeded the 1 MiB limit")
	}
	var payloads []cbPayload
	if json.Unmarshal(stdout.Bytes(), &payloads) != nil || len(payloads) == 0 {
		return failure("provider_status", "codexbar returned invalid or empty usage JSON")
	}

	var selected *cbPayload
	for i := range payloads {
		if payloads[i].Provider == provider {
			if selected != nil {
				return failure("provider_status", "codexbar returned multiple results for the requested provider")
			}
			selected = &payloads[i]
		}
	}
	if selected == nil {
		return failure("provider_status", "codexbar returned no matching provider")
	}
	snap := normalizePayload(*selected)
	if snap.Failure != nil {
		code := "provider_status"
		message := "Approved CodexBar reported a provider failure"
		switch snap.Failure.Code {
		case "unauthorized", "login_required", "expired_credential", "access_denied", "device_expired", "cookie_unavailable", "signing_failed", "keychain_unavailable":
			code = snap.Failure.Code
			message = "Approved CodexBar authentication evidence is unavailable"
		case "timeout":
			code = "timeout"
			message = "codexbar usage request timed out"
		}
		return failure(code, message)
	}
	if runErr != nil {
		return failure("provider_status", "codexbar usage command failed")
	}
	actual, known := companySource(provider, selected.Source)
	if !known {
		return failure("provider_status", "codexbar returned an unsupported source")
	}
	snap.Source = actual
	if !CompanySourceMatches(provider, snap.Source, source) {
		return failure("provider_status", "codexbar returned a different source than requested")
	}
	if selected.Usage == nil {
		return failure("provider_status", "codexbar returned no allowance data")
	}
	recorded := parseTime(selected.Usage.UpdatedAt)
	if recorded.IsZero() || recorded.After(time.Now()) {
		snap.FetchedAt = recorded
		snap.Stale = true
	}
	return snap, nil
}
