//go:build !nousage

package whichmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
)

// UsageArgs is the fully-parsed, validated command input.
type UsageArgs struct {
	Providers    []string
	All          bool
	Source       usage.Source
	MaxAge       time.Duration
	ForceRefresh bool
	Timeout      time.Duration
	Offline      bool
	ShowIdentity bool
	JSON         bool
	ConfigPath   string
	NoUsage      bool
}

// FetchAllOptions is the command-to-fetch boundary.
type FetchAllOptions struct {
	Providers       []string
	Source          usage.Source
	ForceRefresh    bool
	MaxAge          time.Duration
	Timeout         time.Duration
	Offline         bool
	IncludeIdentity bool
}

// FetchResult is the normalized result consumed by the command renderer.
type FetchResult struct {
	Snapshots   []usage.Snapshot
	LastVerified map[string]time.Time
}

var fetchAllFunc = fetchAll

func fetchAll(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
	enabled := make(map[string]bool, len(opts.Providers))
	for _, id := range opts.Providers {
		enabled[id] = true
	}
	snapshots, _, err := fetch.FetchAll(ctx, opts.Providers, fetch.Options{
		Refresh:      opts.ForceRefresh,
		Offline:      opts.Offline || opts.Source == usage.SourceCache,
		MaxAge:       opts.MaxAge,
		ShowIdentity: opts.IncludeIdentity,
		Enabled:      enabled,
		Timeout:      opts.Timeout,
	})
	if err != nil {
		return nil, err
	}
	return &FetchResult{Snapshots: snapshots}, nil
}

func RunUsage(args UsageArgs, stdout, stderr io.Writer) error {
	if args.NoUsage {
		return &CodedError{Code: "usage_disabled", Message: "usage is disabled by --no-usage"}
	}
	if len(args.Providers) > 0 && args.All {
		return &UsageError{Message: "--all and provider arguments are mutually exclusive"}
	}
	if len(args.Providers) == 0 && !args.All {
		return &UsageError{Message: "no providers requested; name providers or pass --all"}
	}
	for _, id := range args.Providers {
		if _, err := usage.Get(id); err != nil {
			return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", id, strings.Join(validUsageIDs(), ", "))}
		}
	}
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	if cfg.Usage.Enabled == config.UsageFalse {
		path := args.ConfigPath
		if path == "" {
			path = "resolved config"
		}
		return &CodedError{Code: "usage_disabled", Message: fmt.Sprintf("usage is disabled by [usage] enabled = false in %s", path)}
	}
	providers, err := resolveProviders(args, cfg)
	if err != nil {
		return err
	}
	if err := validateSource(args.Source); err != nil {
		return &UsageError{Message: err.Error()}
	}
	for _, id := range providers {
		if err := validateProviderSource(id, args.Source); err != nil {
			return &UsageError{Message: err.Error()}
		}
	}
	res, err := fetchAllFunc(context.Background(), FetchAllOptions{
		Providers:       providers,
		Source:          args.Source,
		ForceRefresh:    args.ForceRefresh,
		MaxAge:          args.MaxAge,
		Timeout:         args.Timeout,
		Offline:         args.Offline,
		IncludeIdentity: args.ShowIdentity,
	})
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	res = redactIdentity(res, args.ShowIdentity)
	if res == nil {
		res = &FetchResult{}
	}
	if classified := classifyExit(res.Snapshots); classified != nil {
		for _, snap := range res.Snapshots {
			if snap.Failure == nil {
				continue
			}
			if stderr != nil {
				_, _ = fmt.Fprintf(stderr, "[%s] %s\n", snap.Failure.Code, snap.Failure.Message)
			}
		}
		return classified
	}
	report := reportFromResult(res)
	if args.JSON {
		return emitJSON(report, stdout)
	}
	if stdout != nil {
		_, err = io.WriteString(stdout, FormatUsageText(report, args.ShowIdentity))
		return err
	}
	return nil
}

func reportFromResult(res *FetchResult) *UsageReport {
	report := &UsageReport{SchemaVersion: "2.0", UsageEnabled: true}
	if res == nil {
		report.Snapshots = []usage.Snapshot{}
		return report
	}
	report.Snapshots = res.Snapshots
	report.LastVerified = res.LastVerified
	if report.Snapshots == nil {
		report.Snapshots = []usage.Snapshot{}
	}
	return report
}

func emitJSON(report *UsageReport, stdout io.Writer) error {
	if stdout == nil {
		return nil
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func redactIdentity(res *FetchResult, show bool) *FetchResult {
	if res == nil || show {
		return res
	}
	copyResult := *res
	copyResult.Snapshots = make([]usage.Snapshot, len(res.Snapshots))
	for i, snap := range res.Snapshots {
		snap.Account = ""
		copyResult.Snapshots[i] = snap
	}
	return &copyResult
}

type UsageReport struct {
	SchemaVersion string                 `json:"schema_version"`
	UsageEnabled  bool                   `json:"usage_enabled"`
	Snapshots     []usage.Snapshot       `json:"snapshots"`
	LastVerified  map[string]time.Time   `json:"last_verified,omitempty"`
}

func validUsageIDs() []string { return usage.IDs() }

func resolveProviders(args UsageArgs, cfg *config.Config) ([]string, error) {
	if !args.All {
		return append([]string(nil), args.Providers...), nil
	}
	providers := make([]string, 0)
	for _, id := range usage.IDs() {
		if cfg != nil && cfg.Providers[id].Enabled {
			providers = append(providers, id)
		}
	}
	return providers, nil
}

func displayName(id string) string {
	d, err := usage.Get(id)
	if err != nil || d.DisplayName == "" {
		return id
	}
	return d.DisplayName
}
var validSources = []usage.Source{usage.SourceOAuth, usage.SourceAPI, usage.SourceCLI, usage.SourceWeb, usage.SourceLocal, usage.SourceCache}

func validateSource(source usage.Source) error {
	if source == "" {
		return nil
	}
	for _, valid := range validSources {
		if source == valid {
			return nil
		}
	}
	return fmt.Errorf(`invalid --source %q; valid: oauth, api, cli, web, local, cache`, source)
}

func validateProviderSource(providerID string, source usage.Source) error {
	if source == "" || source == usage.SourceCache {
		return nil
	}
	desc, err := usage.Get(providerID)
	if err != nil {
		return fmt.Errorf("unknown provider %q; valid providers: %s", providerID, strings.Join(validUsageIDs(), ", "))
	}
	declared := make([]usage.Source, 0)
	seen := make(map[usage.Source]bool)
	for _, auth := range desc.Auth {
		mapped := usage.SourceAPI
		switch auth.Kind {
		case usage.AuthOAuthDeviceFlow, usage.AuthOAuthRefreshGrant:
			mapped = usage.SourceOAuth
		case usage.AuthCLIShellOut, usage.AuthSubprocessRPC:
			mapped = usage.SourceCLI
		case usage.AuthBrowserCookie:
			mapped = usage.SourceWeb
		}
		if !seen[mapped] {
			seen[mapped] = true
			declared = append(declared, mapped)
		}
	}
	for _, candidate := range declared {
		if candidate == source {
			return nil
		}
	}
	names := make([]string, len(declared))
	for i, candidate := range declared {
		names[i] = string(candidate)
	}
	return fmt.Errorf(`provider %q has no %s source; valid sources: %s`, providerID, source, strings.Join(names, ", "))
}

var usageExitFiveCodes = map[string]bool{
	"unauthorized":       true,
	"login_required":     true,
	"expired_credential": true,
	"credential_file":    true,
	"credential_json":    true,
	"unsafe_credential":  true,
	"access_denied":      true,
	"device_expired":     true,
	"cookie_unavailable": true,
	"signing_failed":     true,
}

func classifyExit(snaps []usage.Snapshot) *CodedError {
	var first *usage.Failure
	var firstAuth *usage.Failure
	for i := range snaps {
		failure := snaps[i].Failure
		if failure == nil {
			return nil
		}
		if first == nil {
			first = failure
		}
		if firstAuth == nil && usageExitFiveCodes[failure.Code] {
			firstAuth = failure
		}
	}
	if firstAuth != nil {
		return &CodedError{Code: firstAuth.Code, Message: firstAuth.Message}
	}
	if first != nil {
		return &CodedError{Code: first.Code, Message: first.Message}
	}
	return nil
}

func FormatUsageText(report *UsageReport, showIdentity bool) string {
	if report == nil || len(report.Snapshots) == 0 {
		return ""
	}
	var b strings.Builder
	for i, snap := range report.Snapshots {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(displayName(snap.Provider))
		b.WriteString(" usage allowance\n")
		for _, window := range snap.Windows {
			b.WriteString("- ")
			b.WriteString(window.Label)
			b.WriteString(": ")
			details := formatWindowDetails(window)
			b.WriteString(strings.Join(details, "; "))
			b.WriteByte('\n')
		}
		if showIdentity && snap.Account != "" {
			b.WriteString("- account: ")
			b.WriteString(snap.Account)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatWindowDetails(window usage.Window) []string {
	if window.Unlimited {
		return []string{"unlimited"}
	}
	details := make([]string, 0, 5)
	if window.UsedPercent != nil {
		details = append(details, formatNumber(*window.UsedPercent)+"% used")
		if window.Remaining == nil {
			details = append(details, formatNumber(100-*window.UsedPercent)+"% available")
		}
	}
	if window.Remaining != nil {
		details = append(details, formatNumber(*window.Remaining)+" remaining")
	}
	if window.Limit != nil {
		details = append(details, formatNumber(*window.Limit)+" total")
	}
	if window.ResetsAt != nil {
		details = append(details, "resets "+window.ResetsAt.Format(time.RFC3339))
	} else if window.ResetHint != "" {
		hint := window.ResetHint
		if !strings.HasPrefix(hint, "resets") {
			hint = "resets " + hint
		}
		details = append(details, hint)
	}
	return details
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
