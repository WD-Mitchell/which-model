//go:build !nousage

package whichmodel

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
)

// AuthResolved is the command-facing credential resolution result. It keeps
// the opaque secret inside the resolver boundary so renderers can derive
// only its fingerprint.
type AuthResolved struct {
	Source    usage.Source
	Secret    string
	ExpiresAt *time.Time
	Account   string
}

var errNoCredential = credential.ErrNotFound

var resolveFirstFunc = resolveFirst

func managedCredentialStore() (credential.ManagedStore, error) {
	cfg, err := config.Load(config.LoadOptions{Path: Global.ConfigPath})
	if err != nil {
		return credential.ManagedStore{}, err
	}
	auth, err := cfg.LoadAuth()
	if err != nil {
		return credential.ManagedStore{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return credential.ManagedStore{}, errors.New("cannot resolve credential storage directory")
	}
	paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)
	return credential.ManagedStore{
		StateDir:    paths.StateDir,
		Keychain:    credential.DefaultKeychain(),
		UseKeychain: auth.UseKeychain,
	}, nil
}

func resolveFirst(providerID string) (AuthResolved, error) {
	desc, err := usage.Get(providerID)
	if err != nil {
		return AuthResolved{}, err
	}
	store, err := managedCredentialStore()
	if err != nil {
		return AuthResolved{}, err
	}
	resolved, _, err := credential.ResolveProvider(context.Background(), providerID, desc.Auth, &http.Client{}, store)
	if err != nil {
		return AuthResolved{}, err
	}
	result := AuthResolved{
		Source: fetch.SourceFor(resolved, desc.Kind),
		Secret: resolved.Token,
	}
	for _, key := range []string{"account", "account_id", "login", "username"} {
		if result.Account == "" {
			result.Account = resolved.Extra[key]
		}
	}
	return result, nil
}

type StatusEntry struct {
	Provider    string     `json:"provider"`
	Status      string     `json:"status"`
	Source      *string    `json:"source"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Fingerprint *string    `json:"fingerprint"`
	Account     string     `json:"account,omitempty"`
}

type AuthStatusReport struct {
	SchemaVersion string        `json:"schema_version"`
	UsageEnabled  bool          `json:"usage_enabled"`
	Providers     []StatusEntry `json:"providers"`
}

var nowFunc = time.Now

func Fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	h := hex.EncodeToString(sum[:])
	return h[:6] + "…" + h[len(h)-4:]
}

func resolveStatuses(args AuthStatusArgs) ([]StatusEntry, error) {
	providers := append([]string(nil), args.Providers...)
	if args.All || len(providers) == 0 {
		cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
		if err != nil {
			return nil, err
		}
		providers, err = resolveProviders(UsageArgs{All: true}, cfg)
		if err != nil {
			return nil, err
		}
	}
	if err := validateAuthProviders(providers); err != nil {
		return nil, err
	}
	entries := make([]StatusEntry, 0, len(providers))
	for _, provider := range providers {
		resolved, err := resolveFirstFunc(provider)
		if errors.Is(err, errNoCredential) {
			entries = append(entries, StatusEntry{Provider: provider, Status: "missing"})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", provider, err)
		}
		source := string(resolved.Source)
		fingerprint := Fingerprint(resolved.Secret)
		status := "ok"
		if resolved.ExpiresAt != nil && !nowFunc().Before(*resolved.ExpiresAt) {
			status = "expired"
		}
		entry := StatusEntry{
			Provider:    provider,
			Status:      status,
			Source:      &source,
			ExpiresAt:   resolved.ExpiresAt,
			Fingerprint: &fingerprint,
		}
		if args.ShowIdentity {
			entry.Account = resolved.Account
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
func authUsageDisabled(noUsage bool, configPath string) error {
	if noUsage {
		return &CodedError{Code: "usage_disabled", Message: "usage is disabled by --no-usage"}
	}
	cfg, err := config.Load(config.LoadOptions{Path: configPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	if cfg.Usage.Enabled == config.UsageFalse {
		path := configPath
		if path == "" {
			path = "resolved config"
		}
		return &CodedError{Code: "usage_disabled", Message: fmt.Sprintf("usage is disabled by [usage] enabled = false in %s", path)}
	}
	switch cfg.Usage.Backend {
	case config.UsageBackendOff, "":
		return &CodedError{Code: "usage_disabled", Message: "usage is disabled by [usage] backend = off"}
	case config.UsageBackendCodexBar:
		return &CodedError{Code: "unsupported", Message: `which-model auth manages native credentials only; CodexBar manages credentials when [usage] backend = "codexbar"`}
	case config.UsageBackendNative:
		return nil
	default:
		return &UsageError{Message: fmt.Sprintf("unknown usage backend %q", cfg.Usage.Backend)}
	}
}

func RunAuthStatus(args AuthStatusArgs, stdout, stderr io.Writer) error {
	if err := authUsageDisabled(args.NoUsage, args.ConfigPath); err != nil {
		return err
	}
	entries, err := resolveStatuses(args)
	if err != nil {
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			return usageErr
		}
		message := redactAuthMessage(err.Error(), "")
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "[runtime] %s\n", message)
		}
		return &CodedError{Code: "runtime", Message: message}
	}
	report := &AuthStatusReport{SchemaVersion: "2.0", UsageEnabled: true, Providers: entries}
	if args.JSON {
		if err := emitAuthJSON(report, stdout); err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
	} else if stdout != nil {
		if _, err := io.WriteString(stdout, formatAuthStatusText(entries, args.ShowIdentity)); err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
	}
	for _, entry := range entries {
		if entry.Status == "expired" {
			return &ReportedError{Err: &CodedError{Code: "expired_credential", Message: "provider(s) without usable credentials; run which-model auth status"}}
		}
	}
	for _, entry := range entries {
		if entry.Status == "missing" {
			return &ReportedError{Err: &CodedError{Code: "login_required", Message: "provider(s) without usable credentials; run which-model auth status"}}
		}
	}
	return nil
}

func emitAuthJSON(report *AuthStatusReport, stdout io.Writer) error {
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

func formatAuthStatusText(entries []StatusEntry, showIdentity bool) string {
	providerWidth, statusWidth := 7, 7
	for _, entry := range entries {
		if len(entry.Provider) > providerWidth {
			providerWidth = len(entry.Provider)
		}
		if len(entry.Status) > statusWidth {
			statusWidth = len(entry.Status)
		}
	}
	var b strings.Builder
	for _, entry := range entries {
		source, fingerprint, expiry := "-", "-", ""
		if entry.Source != nil {
			source = *entry.Source
		}
		if entry.Fingerprint != nil {
			fingerprint = *entry.Fingerprint
		}
		if entry.ExpiresAt != nil {
			prefix := "expires"
			if entry.Status == "expired" {
				prefix = "expired"
			}
			expiry = "(" + prefix + " " + entry.ExpiresAt.Format(time.RFC3339) + ")"
		}
		_, _ = fmt.Fprintf(&b, "%-*s%-*s%-8s", providerWidth+2, entry.Provider, statusWidth+2, entry.Status, source)
		if fingerprint == "-" {
			b.WriteString(fingerprint)
			b.WriteString(strings.Repeat(" ", 12))
		} else {
			b.WriteString(fingerprint)
			if expiry != "" || entry.Status == "missing" || (showIdentity && entry.Account != "") {
				b.WriteString(strings.Repeat(" ", 3))
			}
		}
		if expiry != "" {
			b.WriteString(expiry)
		} else if entry.Status == "missing" {
			b.WriteString("-    run: which-model auth login ")
			b.WriteString(entry.Provider)
		}
		if showIdentity && entry.Account != "" {
			b.WriteString("    (account ")
			b.WriteString(entry.Account)
			b.WriteByte(')')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

var authSecretPattern = regexp.MustCompile(`(?i)\bghp_[A-Za-z0-9_-]+\b`)

func redactAuthMessage(message, secret string) string {
	if secret != "" {
		message = strings.ReplaceAll(message, secret, "***")
	}
	return authSecretPattern.ReplaceAllString(message, "***")
}

type DeviceFlow struct {
	Code            string
	VerificationURI string
	Poll            func() (string, error)
}

var stdinIsTTY = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
var nonInteractiveEnv = func() bool { return os.Getenv("WHICH_MODEL_NONINTERACTIVE") == "1" }

var startDeviceFlowFunc = startDeviceFlow

func startDeviceFlow(provider string) (DeviceFlow, error) {
	desc, err := usage.Get(provider)
	if err != nil {
		return DeviceFlow{}, err
	}
	for _, source := range desc.Auth {
		if source.Kind != usage.AuthOAuthDeviceFlow || source.OAuth == nil {
			continue
		}
		spec := *source.OAuth
		if spec.VerificationURI == "" && provider == "copilot" {
			spec.VerificationURI = "https://github.com/login/device"
		}
		flow := credential.NewDeviceFlow(spec)
		code, err := flow.Start(context.Background())
		if err != nil {
			return DeviceFlow{}, err
		}
		return DeviceFlow{
			Code:            code.UserCode,
			VerificationURI: code.VerificationURI,
			Poll: func() (string, error) {
				return flow.Poll(context.Background(), code)
			},
		}, nil
	}
	return DeviceFlow{}, &CodedError{Code: "unsupported", Message: fmt.Sprintf("login for %s is not supported until M5; sign in with the provider's own client, then run which-model auth status %s", provider, provider)}
}

var saveCredentialFunc = saveCredential

func saveCredential(provider, token string) error {
	store, err := managedCredentialStore()
	if err != nil {
		return err
	}
	return store.Save(provider, token)
}

func RunAuthLogin(provider string, stdout, stderr io.Writer, stdin io.Reader) error {
	if err := authUsageDisabled(Global.NoUsage, Global.ConfigPath); err != nil {
		return err
	}
	if !stdinIsTTY() || nonInteractiveEnv() {
		return &UsageError{Message: "refusing unattended login; run from an interactive terminal"}
	}
	if _, err := usage.Get(provider); err != nil {
		return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", provider, strings.Join(usage.IDs(), ", "))}
	}
	if provider != "copilot" {
		return &CodedError{Code: "unsupported", Message: fmt.Sprintf("login for %s is not supported until M5; sign in with the provider's own client, then run which-model auth status %s", provider, provider)}
	}
	flow, err := startDeviceFlowFunc(provider)
	if err != nil {
		message := redactAuthMessage(err.Error(), "")
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "[runtime] %s\n", message)
		}
		return &CodedError{Code: "runtime", Message: message}
	}
	if stdout != nil {
		_, _ = fmt.Fprintf(stdout, "Open %s and enter code %s.\n", flow.VerificationURI, flow.Code)
	}
	if stderr != nil {
		_, _ = io.WriteString(stderr, "waiting for confirmation...\n")
	}
	if flow.Poll == nil {
		return &CodedError{Code: "runtime", Message: "device login could not be completed"}
	}
	token, err := flow.Poll()
	if err != nil {
		message := redactAuthMessage(err.Error(), "")
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "[runtime] %s\n", message)
		}
		return &CodedError{Code: "runtime", Message: message}
	}
	if err := saveCredentialFunc(provider, token); err != nil {
		message := redactAuthMessage(err.Error(), token)
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "[runtime] %s\n", message)
		}
		return &CodedError{Code: "runtime", Message: message}
	}
	return nil
}

var removeFunc = removeCredential
var hasBroadPermsFunc = security.HasBroadPermissions
var managedCredentialFileInfoFunc = managedCredentialFileInfo

func removeCredential(provider string) error {
	store, err := managedCredentialStore()
	if err != nil {
		return err
	}
	return store.Remove(provider)
}

func managedCredentialFileInfo(provider string) (string, fs.FileMode, error) {
	store, err := managedCredentialStore()
	if err != nil {
		return "", 0, err
	}
	path := store.Path(provider)
	if path == "" {
		return "", 0, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return path, info.Mode().Perm(), nil
}

func RunAuthLogout(provider string, yes bool, stdout, stderr io.Writer, stdin io.Reader) error {
	if err := authUsageDisabled(Global.NoUsage, Global.ConfigPath); err != nil {
		return err
	}
	if !yes && !stdinIsTTY() {
		return &UsageError{Message: "refusing unattended logout without --yes"}
	}
	if _, err := usage.Get(provider); err != nil {
		return &UsageError{Message: fmt.Sprintf("unknown provider %q; valid providers: %s", provider, strings.Join(usage.IDs(), ", "))}
	}
	if !yes {
		if stdout != nil {
			_, _ = fmt.Fprintf(stdout, "Remove which-model's cached credential for %s? [y/N] ", provider)
		}
		answer, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && len(answer) == 0 {
			answer = ""
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			if stderr != nil {
				_, _ = io.WriteString(stderr, "aborted\n")
			}
			return nil
		}
	}
	path, mode, infoErr := managedCredentialFileInfoFunc(provider)
	if infoErr == nil && path != "" && hasBroadPermsFunc(mode) {
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "Warning: %s permissions are broader than 0600; review them.\n", path)
		}
	}
	if err := removeFunc(provider); err != nil {
		if errors.Is(err, errNoCredential) {
			if stderr != nil {
				_, _ = fmt.Fprintf(stderr, "no which-model-managed credential for %s; nothing to remove\n", provider)
			}
			return nil
		}
		message := redactAuthMessage(err.Error(), "")
		if stderr != nil {
			_, _ = fmt.Fprintf(stderr, "[runtime] %s\n", message)
		}
		return &CodedError{Code: "runtime", Message: message}
	}
	return nil
}
