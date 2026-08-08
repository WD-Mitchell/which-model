//go:build !nousage

// Package credential resolves provider credentials from ordered AuthSource
// chains (specs/features/F12-credentials/SPEC.md): env vars, bounded
// credential files, CLI shell-outs, the OS keychain, and the OAuth
// device-code flow. ResolveChain walks a provider's sources in order and
// returns the first candidate that resolves AND passes Validate.
package credential

import (
	"context"
	"errors"
	"net/http"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Credential is F11's type, re-exported for readable resolver signatures.
// Never log it: String() is redacted (F11).
type Credential = usage.Credential

// Warning is a human-readable, sanitised diagnostic (never credential
// material). Printed by the CLI to stderr (annex-d §1).
type Warning struct {
	Message string
}

// ErrNotFound is the sentinel for "no candidate from this source; continue
// the chain". NOT a *usage.FailureError — compare with errors.Is.
var ErrNotFound = errors.New("credential not found")

// Resolver resolves one credential candidate, or ErrNotFound when this
// source yields no candidate. Any other error is a hard, Failure-coded
// error (usage.NewFailureError) that aborts the chain.
type Resolver interface {
	Resolve(ctx context.Context) (usage.Credential, error)
}

// ResolveChain walks sources in order (SPEC §11). First candidate that
// resolves AND passes Validate wins. Hard errors abort. Exhaustion →
// ErrNotFound (F14 maps it to login_required). Kinds without an F12
// resolver (RPC/refresh/sigv4/volcengine/grpc-web) → ErrNotFound.
func ResolveChain(ctx context.Context, sources []usage.AuthSource, client *http.Client) (usage.Credential, []Warning, error) {
	var warnings []Warning
	for _, source := range sources {
		resolver, ok := resolverFor(source)
		if !ok {
			continue // kind without an F12 resolver → candidate unavailable
		}
		candidate, err := resolver.Resolve(ctx)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			// Hard resolver error: abort the chain immediately (SPEC D11).
			return Credential{}, warnings, err
		}
		if source.Validate != nil {
			if err := source.Validate(ctx, candidate, client); err != nil {
				// A candidate that fails Validate is skipped, never fatal
				// (SPEC D11; Copilot identity-gate semantics).
				continue
			}
		}
		// The winning resolver's warnings ride along (message-only; the
		// text is self-contained, no provider prefix needed).
		if fr, isFile := resolver.(*FileResolver); isFile {
			for _, w := range fr.Warnings() {
				warnings = append(warnings, Warning{Message: w})
			}
		}
		return candidate, warnings, nil
	}
	return Credential{}, warnings, ErrNotFound
}

// resolverFor builds the F12 resolver for an AuthSource kind. ok=false for
// kinds F12 does not resolve (SPEC §11): the chain treats them as
// candidate-unavailable and continues. AuthOAuthDeviceFlow resolves to a
// no-candidate resolver: the flow is interactive — F25's login drives
// Start/Poll directly — and a chain walk must never auto-start it.
func resolverFor(s usage.AuthSource) (Resolver, bool) {
	switch s.Kind {
	case usage.AuthEnvVar:
		return &EnvResolver{Var: s.EnvVar, Extra: s.EnvExtra}, true
	case usage.AuthFile:
		return &FileResolver{Paths: s.FilePaths, JSONPath: s.JSONPath, ExtraPaths: s.ExtraPaths, ExpiryPath: s.ExpiryPath}, true
	case usage.AuthCLIShellOut:
		if s.Shell == nil {
			return nil, false
		}
		return &CLIResolver{Command: s.Shell.Command, Args: s.Shell.Args, Timeout: s.Shell.Timeout}, true
	case usage.AuthKeychainGeneric:
		if s.Keychain == nil {
			return nil, false
		}
		return &KeychainResolver{Store: DefaultKeychain(), Service: s.Keychain.Service, Account: s.Keychain.Account}, true
	case usage.AuthOAuthDeviceFlow:
		return deviceFlowResolver{}, true
	default:
		// AuthBrowserCookie (SPEC D3), AuthKeychainInternet, and the
		// F15–F17 kinds (RPC/refresh/sigv4/volcengine/grpc-web).
		return nil, false
	}
}

// deviceFlowResolver marks AuthOAuthDeviceFlow sources candidate-
// unavailable during chain walks (interactive-only; SPEC §9, D11).
type deviceFlowResolver struct{}

func (deviceFlowResolver) Resolve(ctx context.Context) (usage.Credential, error) {
	return Credential{}, ErrNotFound
}

