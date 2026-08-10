//go:build !nousage

// Package codexbar adapts the locally installed CodexBar CLI to the canonical
// which-model usage snapshot types.
package codexbar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

const (
	defaultTimeout = 30 * time.Second
	maxStdoutBytes = 1 << 20
)

var (
	lookPath         = exec.LookPath
	fixedBinaryPaths = []string{
		"/opt/homebrew/bin/codexbar",
		"/usr/local/bin/codexbar",
		"/Applications/CodexBar.app/Contents/Helpers/CodexBarCLI",
	}
	supportedProvidersOnce sync.Once
	supportedProviders     []string
)

// BinaryNotFoundError reports that no usable CodexBar executable was found.
type BinaryNotFoundError struct{}

func (*BinaryNotFoundError) Error() string { return "codexbar CLI not found" }

// Fetch runs CodexBar for one provider and converts its JSON output into a
// canonical snapshot. Provider failures are returned in Snapshot.Failure;
// binary discovery is the only failure returned as an error.
func Fetch(ctx context.Context, providerID string) (usage.Snapshot, error) {
	return fetchWithSource(ctx, providerID, "")
}

// FetchWithSource is the source-aware form used by the usage command. Fetch
// remains the default two-argument seam for callers that do not force a source.
func FetchWithSource(ctx context.Context, providerID string, source usage.Source) (usage.Snapshot, error) {
	return fetchWithSource(ctx, providerID, source)
}

func fetchWithSource(ctx context.Context, providerID string, source usage.Source) (usage.Snapshot, error) {
	binary, err := findBinary()
	if err != nil {
		return usage.Snapshot{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	cmdCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	args := []string{"usage", "--provider", providerID, "--format", "json", "--json-only", "--no-color"}
	if source != "" && source != usage.SourceCache {
		args = append(args, "--source", string(source))
	}
	cmd := exec.CommandContext(cmdCtx, binary, args...)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Signal(syscall.SIGKILL)
	}
	var stdout cappedBuffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		return failedSnapshot(providerID, usage.SourceAPI, "timeout", "codexbar usage request timed out"), nil
	}
	if cmdCtx.Err() != nil && runErr != nil {
		return failedSnapshot(providerID, usage.SourceAPI, "provider_status", "codexbar usage request was cancelled"), nil
	}
	if stdout.tooLarge {
		return failedSnapshot(providerID, usage.SourceAPI, "response_too_large", "codexbar output exceeded the 1 MiB limit"), nil
	}
	var payloads []cbPayload
	if err := json.Unmarshal(stdout.Bytes(), &payloads); err != nil {
		return failedSnapshot(providerID, usage.SourceAPI, "provider_status", "codexbar returned invalid JSON"), nil
	}
	if len(payloads) == 0 {
		return failedSnapshot(providerID, usage.SourceAPI, "provider_status", "codexbar returned no usage data"), nil
	}

	snapshots := normalizePayloads(payloads)
	for _, snap := range snapshots {
		if snap.Provider == providerID {
			return snap, nil
		}
	}
	if len(snapshots) == 1 {
		snapshots[0].Provider = providerID
		return snapshots[0], nil
	}
	if runErr != nil {
		return failedSnapshot(providerID, usage.SourceAPI, "provider_status", "codexbar usage command failed"), nil
	}
	return failedSnapshot(providerID, usage.SourceAPI, "provider_status", "codexbar returned no matching provider"), nil
}

func findBinary() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CODEXBAR_BIN")); configured != "" && isExecutable(configured) {
		return configured, nil
	}
	if path, err := lookPath("codexbar"); err == nil && path != "" {
		return path, nil
	}
	for _, path := range fixedBinaryPaths {
		if isExecutable(path) {
			return path, nil
		}
	}
	return "", &BinaryNotFoundError{}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// SupportedProviders returns CodexBar's provider enum, falling back to the
// three route providers when CodexBar is unavailable or its help changes.
func SupportedProviders() []string {
	supportedProvidersOnce.Do(func() {
		supportedProviders = discoverSupportedProviders()
	})
	return append([]string(nil), supportedProviders...)
}

func discoverSupportedProviders() []string {
	binary, err := findBinary()
	if err != nil {
		return fallbackProviders()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "usage", "--help").CombinedOutput()
	if err != nil {
		return fallbackProviders()
	}
	providers := parseProviderIDs(string(output))
	if len(providers) == 0 {
		return fallbackProviders()
	}
	return providers
}

func fallbackProviders() []string { return []string{"claude", "codex", "copilot"} }

func parseProviderIDs(help string) []string {
	const marker = "--provider "
	start := strings.Index(help, marker)
	if start < 0 {
		return nil
	}
	value := help[start+len(marker):]
	if end := strings.IndexAny(value, "]\r\n"); end >= 0 {
		value = value[:end]
	}
	seen := make(map[string]struct{})
	providers := make([]string, 0)
	for _, id := range strings.Split(value, "|") {
		id = strings.TrimSpace(strings.Trim(id, "<>"))
		if id == "" || id == "all" || id == "both" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		providers = append(providers, id)
	}
	sort.Strings(providers)
	return providers
}

type cbPayload struct {
	Provider string   `json:"provider"`
	Source   string   `json:"source"`
	Account  string   `json:"account"`
	Usage    *cbUsage `json:"usage"`
	Error    *cbError `json:"error"`
}

type cbUsage struct {
	Primary   *cbRateWindow `json:"primary"`
	Secondary *cbRateWindow `json:"secondary"`
	Tertiary  *cbRateWindow `json:"tertiary"`
	UpdatedAt string        `json:"updatedAt"`
	Identity  *cbIdentity   `json:"identity"`
}

type cbRateWindow struct {
	UsedPercent      *float64 `json:"usedPercent"`
	WindowMinutes    *int     `json:"windowMinutes"`
	ResetsAt         *string  `json:"resetsAt"`
	ResetDescription *string  `json:"resetDescription"`
}

type cbIdentity struct {
	AccountEmail string `json:"accountEmail"`
	LoginMethod  string `json:"loginMethod"`
	ProviderID   string `json:"providerID"`
}

type cbError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func normalizePayloads(payloads []cbPayload) []usage.Snapshot {
	out := make([]usage.Snapshot, 0, len(payloads))
	for _, payload := range payloads {
		out = append(out, normalizePayload(payload))
	}
	return out
}

func normalizePayload(payload cbPayload) usage.Snapshot {
	snap := usage.Snapshot{
		Provider:   payload.Provider,
		Account:    payload.Account,
		Source:     sourceFor(payload.Source),
		Confidence: "live",
	}
	if payload.Error != nil {
		code := payload.Error.Code
		if code == "" {
			code = "provider_status"
		}
		message := payload.Error.Message
		if message == "" {
			message = "codexbar reported a provider error"
		}
		snap.Failure = &usage.Failure{Code: code, Message: message}
	}
	if payload.Usage == nil {
		snap.FetchedAt = time.Now().UTC()
		return snap
	}
	if payload.Usage.Identity != nil {
		snap.Account = payload.Usage.Identity.AccountEmail
		snap.Plan = payload.Usage.Identity.LoginMethod
	}
	snap.FetchedAt = parseTime(payload.Usage.UpdatedAt)
	if snap.FetchedAt.IsZero() {
		snap.FetchedAt = time.Now().UTC()
	}
	for _, item := range []struct {
		window *cbRateWindow
		id     string
		label  string
	}{
		{payload.Usage.Primary, primaryWindowID(payload.Provider), primaryWindowLabel(payload.Provider)},
		{payload.Usage.Secondary, secondaryWindowID(payload.Provider), secondaryWindowLabel(payload.Provider)},
		{payload.Usage.Tertiary, "tertiary", "Tertiary"},
	} {
		if item.window == nil || item.id == "" {
			continue
		}
		window := usage.Window{
			ID:            item.id,
			Label:         item.label,
			Unit:          usage.UnitPercent,
			UsedPercent:   item.window.UsedPercent,
			WindowMinutes: item.window.WindowMinutes,
			ResetHint:     stringValue(item.window.ResetDescription),
			UsageKnown:    item.window.UsedPercent != nil,
		}
		if reset := parseTime(stringValue(item.window.ResetsAt)); !reset.IsZero() {
			window.ResetsAt = &reset
		}
		if window.UsageKnown {
			snap.UsageKnown = true
		}
		snap.Windows = append(snap.Windows, window)
	}
	return snap
}

func sourceFor(source string) usage.Source {
	switch source {
	case "web":
		return usage.SourceWeb
	case "oauth":
		return usage.SourceOAuth
	case "api":
		return usage.SourceAPI
	case "cli":
		return usage.SourceCLI
	case "local":
		return usage.SourceLocal
	default:
		return usage.SourceAPI
	}
}

func primaryWindowID(provider string) string {
	switch provider {
	case "claude":
		return "5h"
	case "codex":
		return "session"
	case "copilot":
		return "monthly"
	default:
		return "primary"
	}
}

func secondaryWindowID(provider string) string {
	switch provider {
	case "claude", "codex":
		return "weekly"
	case "copilot":
		return ""
	default:
		return "secondary"
	}
}

func primaryWindowLabel(provider string) string {
	switch provider {
	case "claude", "codex":
		return "Session"
	case "copilot":
		return "Monthly"
	default:
		return "Primary"
	}
}

func secondaryWindowLabel(provider string) string {
	if provider == "claude" || provider == "codex" {
		return "Weekly"
	}
	return "Secondary"
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func failedSnapshot(provider string, source usage.Source, code, message string) usage.Snapshot {
	return usage.Snapshot{
		Provider:   provider,
		Source:     source,
		Confidence: "live",
		FetchedAt:  time.Now().UTC(),
		Failure:    &usage.Failure{Code: code, Message: message},
	}
}

type cappedBuffer struct {
	bytes.Buffer
	tooLarge bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := maxStdoutBytes - b.Len()
	if remaining <= 0 {
		b.tooLarge = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.tooLarge = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
