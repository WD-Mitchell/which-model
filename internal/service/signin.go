//go:build !nousage

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// SignInService is the interactive device-flow sign-in facet (U-signin).
// It drives the same primitive the CLI's `auth login` drives
// (pkg/whichmodel/auth.go RunAuthLogin → credential.DeviceFlow) so the GUI
// and the CLI produce identical credentials: a ManagedStore entry under
// StateDir (keychain when [auth].use_keychain, else the private state file).
//
// The flow is split in two bound calls so the webview never blocks a render:
// Start returns the verification URL + user code immediately; Confirm does
// the polling (called by the frontend from an effect/worker). At most one
// flow per provider is active at a time; a second Start for the same
// provider restarts it (the previous poll is abandoned).
type SignInService struct{ s *Services }

// SignIn returns the sign-in facet.
func (s *Services) SignIn() *SignInService { return &SignInService{s: s} }

// SignInStart is the Start result: what the user must see to approve.
type SignInStart struct {
	VerificationURI string `json:"verification_uri"` // exact https URL to open
	UserCode        string `json:"user_code"`        // code to enter, display-only
}

// signInFlow carries one active flow between Start and Confirm.
type signInFlow struct {
	flow *credential.DeviceFlow
	code credential.DeviceCode
}

// signInMu guards the active-flows map only. Config access inside
// Start/Confirm follows the same lock discipline as every other method.
var signInMu sync.Mutex

// signInFlows holds the active device flow per provider id.
var signInFlows = map[string]signInFlow{}

// newDeviceFlow builds the flow for one spec. Test seam: signin_test.go
// replaces it (mirroring pkg/whichmodel's startDeviceFlowFunc seam) because
// security.ValidateExactHTTPS requires https and tests run http servers.
var newDeviceFlow = credential.NewDeviceFlow

// Start begins the device flow for provider (currently only "copilot" ships
// one; others return the CLI's own "not supported" text). Usage must be
// enabled — the credential is useless otherwise — mirroring the CLI's gate
// (auth.go authUsageDisabled). The desktop binary never builds nousage
// (S02 SPEC §2.1), so no compiled-out variant of this file is required.
func (g *SignInService) Start(ctx context.Context, provider string) (SignInStart, error) {
	if err := ctx.Err(); err != nil {
		return SignInStart{}, toErrorDTO(err)
	}
	if err := g.signInGate(provider); err != nil {
		return SignInStart{}, toErrorDTO(err)
	}
	desc, err := usage.Get(provider)
	if err != nil {
		return SignInStart{}, toErrorDTO(fmt.Errorf("%w: unknown provider %q", errValidation, provider))
	}
	spec, ok := deviceFlowSpec(desc)
	if !ok {
		return SignInStart{}, toErrorDTO(fmt.Errorf("%w: sign-in for %s is not supported; sign in with the provider's own client, then restart which-model", errValidation, provider))
	}
	flow := newDeviceFlow(spec)
	code, err := flow.Start(ctx)
	if err != nil {
		return SignInStart{}, toErrorDTO(err)
	}
	signInMu.Lock()
	signInFlows[provider] = signInFlow{flow: flow, code: code}
	signInMu.Unlock()
	return SignInStart{VerificationURI: code.VerificationURI, UserCode: code.UserCode}, nil
}

// Confirm polls the active flow for provider until it is approved, expired,
// denied, or ctx is done; on success the token is saved to the managed
// store. It is expected to take tens of seconds (human approval) and is
// therefore called from the frontend off the render path.
func (g *SignInService) Confirm(ctx context.Context, provider string) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	signInMu.Lock()
	active, ok := signInFlows[provider]
	if ok {
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
	if !ok {
		return toErrorDTO(fmt.Errorf("%w: no sign-in in progress for %s; press Sign in first", errValidation, provider))
	}
	token, err := active.flow.Poll(ctx, active.code)
	if err != nil {
		return toErrorDTO(flowErr(err))
	}
	store, err := g.s.managedStoreFor(provider)
	if err != nil {
		return toErrorDTO(err)
	}
	if err := store.Save(provider, token); err != nil {
		return toErrorDTO(err)
	}
	return nil
}

// Cancel abandons an active flow (window closed, user navigated away).
// Removing a non-existent flow is success.
func (g *SignInService) Cancel(provider string) error {
	signInMu.Lock()
	delete(signInFlows, provider)
	signInMu.Unlock()
	return nil
}

// managedStoreFor builds the ManagedStore from the live config, mirroring
// pkg/whichmodel/auth.go managedCredentialStore but with service-injected
// paths (B00 §2.8: no os.UserHomeDir here).
func (s *Services) managedStoreFor(_ string) (credential.ManagedStore, error) {
	s.mu.RLock()
	auth, err := s.cfg.LoadAuth()
	stateDir := s.paths.StateDir
	s.mu.RUnlock()
	if err != nil {
		return credential.ManagedStore{}, err
	}
	return credential.ManagedStore{
		StateDir:    stateDir,
		Keychain:    credential.DefaultKeychain(),
		UseKeychain: auth.UseKeychain,
	}, nil
}

// signInGate mirrors the CLI's usage-enabled gate for the native backend:
// sign-in writes a managed credential, which only the native backend reads.
// CodexBar keeps [auth] untouched by design (auth.go authUsageDisabled).
func (g *SignInService) signInGate(provider string) error {
	known := false
	for _, id := range usage.IDs() {
		if id == provider {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("%w: unknown provider %q", errValidation, provider)
	}
	g.s.mu.RLock()
	cfg := g.s.cfg
	g.s.mu.RUnlock()
	enabled, reason := toggle.ResolveUsageEnabled(false, cfg)
	if enabled {
		return nil
	}
	return fmt.Errorf("%w: usage is disabled (%s); enable usage before signing in", errUsageUnavailable, reason)
}

// deviceFlowSpec extracts the provider's OAuth device-flow spec.
func deviceFlowSpec(desc usage.Descriptor) (usage.OAuthSpec, bool) {
	for _, source := range desc.Auth {
		if source.Kind == usage.AuthOAuthDeviceFlow && source.OAuth != nil {
			return *source.OAuth, true
		}
	}
	return usage.OAuthSpec{}, false
}

// flowErr converts a device-flow error to a sentinel-wrapped error so
// toErrorDTO maps it to a stable code. Messages from the credential layer
// are already sanitised (deviceflow.go constructs them from RFC 8628 error
// codes only, never from response bodies that could carry secrets).
func flowErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: sign-in cancelled", errValidation)
	}
	return err
}
