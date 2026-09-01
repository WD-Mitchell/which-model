//go:build !nousage

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// managedOAuthRef is the account.ref written after a successful device login.
// It names the which-model keychain service (never the token). The settings
// UI treats a non-empty oauth ref as signed in.
const managedOAuthRef = "which-model"

// SignInService is the interactive OAuth sign-in facet (U-signin).
// Copilot uses RFC 8628 device flow; Codex uses OpenAI's device-auth
// endpoints; Claude uses PKCE + a pasted code. All three save a ManagedStore
// entry and, for Claude/Codex, the provider CLI credential file so usage
// fetch keeps working.
//
// Start returns immediately with the URL (and user code when there is one).
// Confirm blocks until approved. Claude Confirm waits for SubmitCode.
type SignInService struct{ s *Services }

// SignIn returns the sign-in facet.
func (s *Services) SignIn() *SignInService { return &SignInService{s: s} }

// SignInStart is the Start result: what the user must see to approve.
type SignInStart struct {
	VerificationURI string `json:"verification_uri"` // exact https URL to open
	UserCode        string `json:"user_code"`        // code to enter; empty for paste flows
}

type signInKind int

const (
	signInRFC8628 signInKind = iota
	signInCodex
	signInClaude
)

// signInFlow carries one active flow between Start and Confirm.
type signInFlow struct {
	kind   signInKind
	flow   *credential.DeviceFlow
	code   credential.DeviceCode
	codex  *codex.DeviceLogin
	claude *claude.BrowserLogin
	pasted chan string
	ctx    context.Context
	cancel context.CancelFunc
}

var signInMu sync.Mutex
var signInFlows = map[string]signInFlow{}

// newDeviceFlow builds the RFC 8628 flow. Test seam: signin_test.go replaces it.
var newDeviceFlow = credential.NewDeviceFlow

var startCodexLogin = func(ctx context.Context) (*codex.DeviceLogin, error) {
	return codex.StartDeviceLogin(ctx, codex.Issuer, codex.ClientID, nil)
}

var startClaudeLogin = func() (*claude.BrowserLogin, error) {
	return claude.StartBrowserLogin()
}

// persistClaudeLogin / persistCodexLogin write the provider CLI files.
// Tests may replace them to avoid touching $HOME.
var persistClaudeLogin = claude.PersistLogin
var persistCodexLogin = codex.PersistLogin

// Start begins sign-in for provider. Usage must be enabled.
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

	pollCtx, cancel := context.WithCancel(context.Background())
	var active signInFlow
	var out SignInStart

	if spec, ok := deviceFlowSpec(desc); ok {
		flow := newDeviceFlow(spec)
		code, err := flow.Start(ctx)
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(err)
		}
		active = signInFlow{kind: signInRFC8628, flow: flow, code: code, ctx: pollCtx, cancel: cancel}
		out = SignInStart{VerificationURI: code.VerificationURI, UserCode: code.UserCode}
	} else if provider == "codex" {
		login, err := startCodexLogin(ctx)
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(err)
		}
		active = signInFlow{kind: signInCodex, codex: login, ctx: pollCtx, cancel: cancel}
		out = SignInStart{VerificationURI: login.VerificationURL, UserCode: login.UserCode}
	} else if provider == "claude" {
		login, err := startClaudeLogin()
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(err)
		}
		active = signInFlow{
			kind:   signInClaude,
			claude: login,
			pasted: make(chan string, 1),
			ctx:    pollCtx,
			cancel: cancel,
		}
		out = SignInStart{VerificationURI: login.AuthorizeURL, UserCode: ""}
	} else {
		cancel()
		return SignInStart{}, toErrorDTO(fmt.Errorf("%w: sign-in for %s is not supported; sign in with the provider's own client, then restart which-model", errValidation, provider))
	}

	signInMu.Lock()
	if prev, ok := signInFlows[provider]; ok && prev.cancel != nil {
		prev.cancel()
	}
	signInFlows[provider] = active
	signInMu.Unlock()
	return out, nil
}

// Confirm waits for the active flow to complete and saves the credential.
func (g *SignInService) Confirm(ctx context.Context, provider string) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	signInMu.Lock()
	active, ok := signInFlows[provider]
	signInMu.Unlock()
	if !ok {
		return toErrorDTO(fmt.Errorf("%w: no sign-in in progress for %s; press Sign in first", errValidation, provider))
	}
	pollCtx := active.ctx
	if pollCtx == nil {
		pollCtx = ctx
	}

	var (
		token   string
		claudeT claude.Tokens
		codexT  codex.Tokens
		err     error
	)
	switch active.kind {
	case signInRFC8628:
		token, err = active.flow.Poll(pollCtx, active.code)
	case signInCodex:
		codexT, err = active.codex.Wait(pollCtx)
		token = codexT.AccessToken
	case signInClaude:
		var pasted string
		select {
		case pasted = <-active.pasted:
			claudeT, err = active.claude.Exchange(pollCtx, pasted)
			token = claudeT.AccessToken
		case <-pollCtx.Done():
			err = pollCtx.Err()
		}
	default:
		err = fmt.Errorf("%w: unknown sign-in kind", errValidation)
	}

	signInMu.Lock()
	if cur, still := signInFlows[provider]; still && sameFlow(cur, active) {
		if cur.cancel != nil {
			cur.cancel()
		}
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
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
	switch provider {
	case "claude":
		_ = persistClaudeLogin(claudeT)
	case "codex":
		_ = persistCodexLogin(codexT)
	}
	_ = g.markManagedOAuth(provider)
	_ = g.s.Providers().RefreshRoutes(ctx)
	g.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	g.s.emit(EventUsageUpdated, struct{}{})
	return nil
}

func sameFlow(a, b signInFlow) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case signInRFC8628:
		return a.flow == b.flow
	case signInCodex:
		return a.codex == b.codex
	case signInClaude:
		return a.claude == b.claude
	default:
		return false
	}
}

// SubmitCode delivers the pasted Claude authentication code to Confirm.
func (g *SignInService) SubmitCode(provider, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return toErrorDTO(fmt.Errorf("%w: paste the code from the login page", errValidation))
	}
	signInMu.Lock()
	active, ok := signInFlows[provider]
	signInMu.Unlock()
	if !ok || active.kind != signInClaude || active.pasted == nil {
		return toErrorDTO(fmt.Errorf("%w: no paste sign-in in progress for %s", errValidation, provider))
	}
	select {
	case active.pasted <- code:
		return nil
	default:
		return toErrorDTO(fmt.Errorf("%w: the code was already submitted", errValidation))
	}
}

// Cancel abandons an active flow.
func (g *SignInService) Cancel(provider string) error {
	signInMu.Lock()
	if active, ok := signInFlows[provider]; ok {
		if active.cancel != nil {
			active.cancel()
		}
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
	return nil
}

func (g *SignInService) markManagedOAuth(provider string) error {
	g.s.mu.Lock()
	defer g.s.mu.Unlock()
	if g.s.cfg == nil {
		return nil
	}
	copyCfg, cleanup, err := cloneConfigForProviders(g.s.cfg)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}
	if copyCfg.Providers == nil {
		copyCfg.Providers = map[string]config.ProviderConfig{}
	}
	providerCfg := copyCfg.Providers[provider]
	accounts := make([]config.ProviderAccount, len(providerCfg.Accounts))
	copy(accounts, providerCfg.Accounts)
	changed := false
	for i := range accounts {
		if accounts[i].Kind == AccountKindOAuth {
			accounts[i].Ref = managedOAuthRef
			changed = true
		}
	}
	if !changed {
		name := provider
		if desc, err := usage.Get(provider); err == nil && desc.DisplayName != "" {
			name = desc.DisplayName
		}
		accounts = append(accounts, config.ProviderAccount{Name: name, Kind: AccountKindOAuth, Ref: managedOAuthRef})
	}
	if _, existed := copyCfg.Providers[provider]; !existed {
		providerCfg.Enabled = true
	}
	providerCfg.Accounts = accounts
	copyCfg.Providers[provider] = providerCfg
	return g.s.Providers().persistConfigLocked(copyCfg)
}

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

func deviceFlowSpec(desc usage.Descriptor) (usage.OAuthSpec, bool) {
	for _, source := range desc.Auth {
		if source.Kind == usage.AuthOAuthDeviceFlow && source.OAuth != nil {
			return *source.OAuth, true
		}
	}
	return usage.OAuthSpec{}, false
}

func flowErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: sign-in cancelled", errValidation)
	}
	return err
}
