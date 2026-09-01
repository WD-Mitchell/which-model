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
	"github.com/WD-Mitchell/which-model/internal/pick/band"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
)

// UsageArgs is the fully-parsed, validated command input.
type UsageArgs struct {
	Providers     []string
	All           bool
	Source        usage.Source
	BandAtOrAbove string
	MaxAge        time.Duration
	ForceRefresh  bool
	Timeout       time.Duration
	Offline       bool
	ShowIdentity  bool
	JSON          bool
	ConfigPath    string
	NoUsage       bool
}

// FetchAllOptions is the command-to-fetch boundary.
type FetchAllOptions struct {
	Providers              []string
	Source                 usage.Source
	Backend                config.UsageBackend
	ForceRefresh           bool
	MaxAge                 time.Duration
	Timeout                time.Duration
	Offline                bool
	IncludeIdentity        bool
	DisableManagedKeychain bool
}

// FetchResult is the normalized result consumed by the command renderer.
type FetchResult struct {
	Snapshots    []usage.Snapshot
	LastVerified map[string]time.Time
}

var fetchAllFunc = fetchAll

func fetchAll(ctx context.Context, opts FetchAllOptions) (*FetchResult, error) {
	enabled := make(map[string]bool, len(opts.Providers))
	for _, id := range opts.Providers {
		enabled[id] = true
	}
	snapshots, _, err := fetch.FetchAll(ctx, opts.Providers, fetch.Options{
		Backend:                opts.Backend,
		Refresh:                opts.ForceRefresh,
		Offline:                opts.Offline || opts.Source == usage.SourceCache,
		MaxAge:                 opts.MaxAge,
		ShowIdentity:           opts.IncludeIdentity,
		Enabled:                enabled,
		Timeout:                opts.Timeout,
		Source:                 opts.Source,
		DisableManagedKeychain: opts.DisableManagedKeychain,
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
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	auth, err := cfg.LoadAuth()
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	bandThreshold, err := newUsageBandThreshold(cfg, args.BandAtOrAbove)
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	validIDs := validUsageIDsForBackend(cfg.Usage.Backend)
	for _, id := range args.Providers {
		found := false
		for _, valid := range validIDs {
			if id == valid {
				found = true
				break
			}
		}
		if !found {
			return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", id, strings.Join(validIDs, ", "))}
		}
	}
	if cfg.Usage.Enabled == config.UsageFalse {
		path := args.ConfigPath
		if path == "" {
			path = "resolved config"
		}
		return &CodedError{Code: "usage_disabled", Message: fmt.Sprintf("usage is disabled by [usage] enabled = false in %s", path)}
	}
	if cfg.Usage.Backend == config.UsageBackendOff || cfg.Usage.Backend == "" {
		return &CodedError{Code: "usage_disabled", Message: "usage is disabled by [usage] backend = off"}
	}
	providers, err := resolveProviders(args, cfg)
	if err != nil {
		return err
	}
	if err := validateSource(args.Source); err != nil {
		return &UsageError{Message: err.Error()}
	}
	for _, id := range providers {
		if err := validateProviderSourceForBackend(id, args.Source, cfg.Usage.Backend); err != nil {
			return &UsageError{Message: err.Error()}
		}
	}
	res, err := fetchAllFunc(context.Background(), FetchAllOptions{
		Providers:              providers,
		Source:                 args.Source,
		Backend:                cfg.Usage.Backend,
		ForceRefresh:           args.ForceRefresh,
		MaxAge:                 args.MaxAge,
		Timeout:                args.Timeout,
		Offline:                args.Offline,
		IncludeIdentity:        args.ShowIdentity,
		DisableManagedKeychain: !auth.UseKeychain,
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
	if bandThreshold != nil {
		res.Snapshots = filterUsageSnapshots(res.Snapshots, bandThreshold)
		for provider := range res.LastVerified {
			found := false
			for i := range res.Snapshots {
				if res.Snapshots[i].Provider == provider {
					found = true
					break
				}
			}
			if !found {
				delete(res.LastVerified, provider)
			}
		}
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

type usageBandThreshold struct {
	config  band.Config
	minimum int
}

func newUsageBandThreshold(cfg *config.Config, name string) (*usageBandThreshold, error) {
	if name == "" {
		return nil, nil
	}
	var raw band.TOMLConfig
	if err := cfg.UnmarshalKey("bands", &raw); err != nil {
		return nil, err
	}
	bandConfig, err := band.FromTOML(raw)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(bandConfig.Tiers))
	for i := range bandConfig.Tiers {
		names[i] = bandConfig.Tiers[i].Name
		if names[i] == name {
			return &usageBandThreshold{config: bandConfig, minimum: i}, nil
		}
	}
	return nil, fmt.Errorf("invalid --band-at-or-above %q; valid: %s", name, strings.Join(names, ", "))
}

func filterUsageSnapshots(snapshots []usage.Snapshot, threshold *usageBandThreshold) []usage.Snapshot {
	kept := snapshots[:0]
	for i := range snapshots {
		if threshold.matches(snapshots[i]) {
			kept = append(kept, snapshots[i])
		}
	}
	return kept
}

func (threshold *usageBandThreshold) matches(snapshot usage.Snapshot) bool {
	if snapshot.Failure != nil {
		return false
	}
	pressure := band.Pressure{}
	for i := range snapshot.Windows {
		percent, ok := band.WindowPercent(snapshot.Windows[i])
		if !ok {
			continue
		}
		if !pressure.Known || percent.GreaterThan(pressure.Percent) {
			pressure.Known = true
			pressure.Percent = percent
		}
	}
	if !pressure.Known {
		return false
	}
	result := band.EvaluateBand(pressure, threshold.config)
	if result.Gated {
		return true
	}
	for i := range threshold.config.Tiers {
		if threshold.config.Tiers[i].Name == result.Name {
			return i >= threshold.minimum
		}
	}
	return false
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
		snap.Plan = ""
		copyResult.Snapshots[i] = snap
	}
	return &copyResult
}

type UsageReport struct {
	SchemaVersion string               `json:"schema_version"`
	UsageEnabled  bool                 `json:"usage_enabled"`
	Snapshots     []usage.Snapshot     `json:"snapshots"`
	LastVerified  map[string]time.Time `json:"last_verified,omitempty"`
}

var nativeUsageIDs = []string{"claude", "codex", "copilot"}

func validUsageIDs() []string { return codexbar.SupportedProviders() }

func validUsageIDsForBackend(backend config.UsageBackend) []string {
	if backend == config.UsageBackendCodexBar {
		return validUsageIDs()
	}
	return append([]string(nil), nativeUsageIDs...)
}

func resolveProviders(args UsageArgs, cfg *config.Config) ([]string, error) {
	if !args.All {
		return append([]string(nil), args.Providers...), nil
	}
	backend := config.UsageBackendNative
	if cfg != nil {
		backend = cfg.Usage.Backend
	}
	providers := make([]string, 0)
	for _, id := range validUsageIDsForBackend(backend) {
		if cfg != nil && cfg.Providers[id].Enabled {
			providers = append(providers, id)
		}
	}
	return providers, nil
}

func displayName(id string) string { return id }

// validSources is the closed --source vocabulary (F24 SPEC §2.4, D-1). The
// empty value is the auto fallback chain and is validated separately.
var validSources = []usage.Source{usage.SourceOAuth, usage.SourceAPI, usage.SourceCLI, usage.SourceWeb, usage.SourceLocal, usage.SourceCache}

func joinSources(sources []usage.Source) string {
	parts := make([]string, len(sources))
	for i, s := range sources {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}

func validateSource(source usage.Source) error {
	if source == "" {
		return nil
	}
	for _, valid := range validSources {
		if source == valid {
			return nil
		}
	}
	return fmt.Errorf("invalid --source %q; valid: %s", source, joinSources(validSources))
}

// validateProviderSource rejects a forced source the provider's credential
// chain cannot produce. The empty value is the auto chain and cache is a
// universal view (every provider reports from cache, D-7), so both skip the
// membership check.
func validateProviderSource(providerID string, source usage.Source) error {
	return validateProviderSourceForBackend(providerID, source, config.UsageBackendNative)
}

// validateProviderSourceForBackend validates a forced source against the
// provider capabilities of the SELECTED backend (issue #28 review P2): the
// native registry is only consulted for the native backend; under codexbar
// the provider id list comes from CodexBar discovery and its supported
// source set applies (codexbar reports normalized percent windows, so
// oauth/api/cli/web all describe its credential surface; cache and empty
// remain universal).
func validateProviderSourceForBackend(providerID string, source usage.Source, backend config.UsageBackend) error {
	if source == "" || source == usage.SourceCache {
		return nil
	}
	if backend != config.UsageBackendNative {
		return nil // codexbar providers are dynamic; no per-provider source registry exists
	}
	desc, err := usage.Get(providerID)
	if err != nil {
		return fmt.Errorf("unknown provider %q", providerID)
	}
	declared := desc.AuthSources()
	for _, valid := range declared {
		if source == valid {
			return nil
		}
	}
	return fmt.Errorf("provider %q has no %s source; valid sources: %s", providerID, source, joinSources(declared))
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
