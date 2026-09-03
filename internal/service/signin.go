//go:build !nousage

package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/antigravity"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/cursor"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// OAuth account refs identify the component that owns the credential. They are
// non-secret markers used by the settings UI to determine sign-in state.
const (
	managedOAuthRef = "which-model"
	cursorOAuthRef  = "cursor-agent"
)

// SignInService is the interactive OAuth sign-in facet (U-signin).
// Copilot uses RFC 8628 device flow; Codex uses OpenAI's device-auth
// endpoints; Claude uses PKCE + a pasted code; Antigravity uses Google OAuth
// with a loopback callback; and Cursor delegates credential persistence to
// Cursor Agent. Every flow records a non-secret account reference in config.
//
// Start returns immediately with the URL (and user code when there is one).
// Confirm blocks until approved. Claude Confirm waits for SubmitCode.
type SignInService struct{ s *Services }

// SignIn returns the sign-in facet.
func (s *Services) SignIn() *SignInService { return &SignInService{s: s} }

// SignInStart is the Start result: what the user must see to approve.
type SignInStart struct {
	FlowID          string `json:"flow_id"`          // unguessable id required by follow-up operations
	VerificationURI string `json:"verification_uri"` // exact https URL to open
	UserCode        string `json:"user_code"`        // device code to enter; empty for browser flows
	PasteRequired   bool   `json:"paste_required"`   // true only when Confirm awaits SubmitCode
}

type signInKind int

const (
	signInRFC8628 signInKind = iota
	signInCodex
	signInClaude
	signInAntigravity
	signInCursor
)

// signInFlow carries one active flow between Start and Confirm.
type signInFlow struct {
	id          string
	kind        signInKind
	flow        *credential.DeviceFlow
	code        credential.DeviceCode
	codex       *codex.DeviceLogin
	claude      *claude.BrowserLogin
	antigravity *antigravitySignIn
	cursor      *cursorSignIn
	pasted      chan string
	ctx         context.Context
	cancel      context.CancelFunc
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

type antigravitySignIn struct {
	verificationURL string
	wait            func(context.Context) (string, error)
}

type cursorSignIn struct {
	verificationURL string
	wait            func(context.Context) error
}

var startAntigravityLogin = func(ctx context.Context) (*antigravitySignIn, error) {
	login, err := antigravity.StartBrowserLogin(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &antigravitySignIn{
		verificationURL: login.VerificationURL,
		wait: func(waitCtx context.Context) (string, error) {
			credentials, err := login.Wait(waitCtx)
			if err != nil {
				return "", err
			}
			return antigravity.EncodeCredential(credentials)
		},
	}, nil
}

var startCursorLogin = func(ctx context.Context) (*cursorSignIn, error) {
	login, err := cursor.StartBrowserLogin(ctx)
	if err != nil {
		return nil, err
	}
	return &cursorSignIn{verificationURL: login.VerificationURL, wait: login.Wait}, nil
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
	signInMu.Lock()
	_, alreadyActive := signInFlows[provider]
	signInMu.Unlock()
	if alreadyActive {
		return SignInStart{}, toErrorDTO(fmt.Errorf("%w: sign-in already in progress for %s", errConflict, provider))
	}

	pollCtx, cancel := context.WithCancel(context.Background())
	var active signInFlow
	var out SignInStart

	switch provider {
	case "antigravity":
		login, err := startAntigravityLogin(pollCtx)
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(err)
		}
		active = signInFlow{kind: signInAntigravity, antigravity: login, ctx: pollCtx, cancel: cancel}
		out = SignInStart{VerificationURI: login.verificationURL}
	case "cursor":
		login, err := startCursorLogin(pollCtx)
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(err)
		}
		active = signInFlow{kind: signInCursor, cursor: login, ctx: pollCtx, cancel: cancel}
		out = SignInStart{VerificationURI: login.verificationURL}
	default:
		desc, err := usage.Get(provider)
		if err != nil {
			cancel()
			return SignInStart{}, toErrorDTO(fmt.Errorf("%w: unknown provider %q", errValidation, provider))
		}
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
			out = SignInStart{VerificationURI: login.AuthorizeURL, PasteRequired: true}
		} else {
			cancel()
			return SignInStart{}, toErrorDTO(fmt.Errorf("%w: sign-in for %s is not supported; sign in with the provider's own client, then restart which-model", errValidation, provider))
		}
	}

	active.id = rand.Text()
	out.FlowID = active.id
	signInMu.Lock()
	if _, exists := signInFlows[provider]; exists {
		signInMu.Unlock()
		cancel()
		return SignInStart{}, toErrorDTO(fmt.Errorf("%w: sign-in already in progress for %s", errConflict, provider))
	}
	signInFlows[provider] = active
	signInMu.Unlock()
	return out, nil
}

// Confirm waits for the active flow to complete, saves the credential, and
// associates it with accountName in provider settings.
func (g *SignInService) Confirm(ctx context.Context, provider, flowID, accountName string) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	signInMu.Lock()
	active, ok := signInFlows[provider]
	signInMu.Unlock()
	if !ok || active.id != flowID {
		return toErrorDTO(fmt.Errorf("%w: no matching sign-in in progress for %s; press Sign in first", errValidation, provider))
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
	case signInAntigravity:
		token, err = active.antigravity.wait(pollCtx)
	case signInCursor:
		err = active.cursor.wait(pollCtx)
	default:
		err = fmt.Errorf("%w: unknown sign-in kind", errValidation)
	}

	signInMu.Lock()
	cur, still := signInFlows[provider]
	current := still && cur.id == flowID
	if current {
		if cur.cancel != nil {
			cur.cancel()
		}
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
	if !current {
		if err != nil {
			return toErrorDTO(flowErr(err))
		}
		return toErrorDTO(fmt.Errorf("%w: sign-in attempt changed for %s; result ignored", errValidation, provider))
	}
	if err != nil {
		return toErrorDTO(flowErr(err))
	}

	accountRef := managedOAuthRef
	if active.kind == signInCursor {
		// Cursor Agent owns and persists its session. Recording a sentinel in
		// which-model's credential store would shadow the real client state.
		accountRef = cursorOAuthRef
		if err := g.recordOAuthAccount(provider, accountName, accountRef); err != nil {
			return toErrorDTO(err)
		}
	} else {
		store, err := g.s.managedStoreFor(provider)
		if err != nil {
			return toErrorDTO(err)
		}
		previous, _, previousErr := store.Resolve(ctx, provider)
		if previousErr != nil && !errors.Is(previousErr, credential.ErrNotFound) {
			return toErrorDTO(previousErr)
		}
		if err := store.Save(provider, token); err != nil {
			return toErrorDTO(err)
		}
		if err := g.recordOAuthAccount(provider, accountName, accountRef); err != nil {
			if restoreErr := restoreManagedCredential(store, provider, previous, previousErr == nil); restoreErr != nil {
				err = fmt.Errorf("%w; managed credential restoration failed: %w", err, restoreErr)
			}
			return toErrorDTO(err)
		}
		switch provider {
		case "claude":
			_ = persistClaudeLogin(claudeT)
		case "codex":
			_ = persistCodexLogin(codexT)
		}
	}
	_ = g.s.Providers().RefreshRoutes(ctx)
	g.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	g.s.emit(EventUsageUpdated, struct{}{})
	return nil
}

// SubmitCode delivers the pasted Claude authentication code to Confirm.
func (g *SignInService) SubmitCode(provider, flowID, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return toErrorDTO(fmt.Errorf("%w: paste the code from the login page", errValidation))
	}
	signInMu.Lock()
	active, ok := signInFlows[provider]
	signInMu.Unlock()
	if !ok || active.id != flowID || active.kind != signInClaude || active.pasted == nil {
		return toErrorDTO(fmt.Errorf("%w: no matching paste sign-in in progress for %s", errValidation, provider))
	}
	select {
	case active.pasted <- code:
		return nil
	default:
		return toErrorDTO(fmt.Errorf("%w: the code was already submitted", errValidation))
	}
}

// Cancel abandons only the named flow. A stale cancel cannot terminate a
// replacement sign-in for the same provider.
func (g *SignInService) Cancel(provider, flowID string) error {
	signInMu.Lock()
	active, ok := signInFlows[provider]
	if ok && active.id != flowID {
		signInMu.Unlock()
		return toErrorDTO(fmt.Errorf("%w: sign-in attempt changed for %s; cancel ignored", errValidation, provider))
	}
	if ok {
		if active.cancel != nil {
			active.cancel()
		}
		delete(signInFlows, provider)
	}
	signInMu.Unlock()
	return nil
}

func (g *SignInService) recordOAuthAccount(provider, accountName, accountRef string) error {
	accountName = strings.TrimSpace(accountName)
	if accountName == "" {
		return fmt.Errorf("%w: providers: an account needs a name", errValidation)
	}
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
	found := false
	for i := range accounts {
		if strings.TrimSpace(accounts[i].Name) != accountName {
			continue
		}
		if accounts[i].Kind != AccountKindOAuth {
			return fmt.Errorf("%w: providers: duplicate account name %q", errConflict, accountName)
		}
		accounts[i].Ref = accountRef
		found = true
		break
	}
	if !found {
		accounts = append(accounts, config.ProviderAccount{
			Name: accountName,
			Kind: AccountKindOAuth,
			Ref:  accountRef,
		})
	}
	if _, existed := copyCfg.Providers[provider]; !existed {
		providerCfg.Enabled = true
	}
	providerCfg.Accounts = accounts
	copyCfg.Providers[provider] = providerCfg
	return g.s.Providers().persistConfigLocked(copyCfg)
}

// SaveAPIKey stores an API key in which-model's managed credential store and
// writes only its non-secret account reference to config.toml.
func (g *SignInService) SaveAPIKey(ctx context.Context, provider, accountName, apiKey string) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	accountName = strings.TrimSpace(accountName)
	if accountName == "" {
		return toErrorDTO(fmt.Errorf("%w: providers: an account needs a name", errValidation))
	}
	store, err := g.s.managedStoreFor(provider)
	if err != nil {
		return toErrorDTO(err)
	}

	g.s.mu.Lock()
	if !g.s.Providers().providerKnownLocked(provider) {
		g.s.mu.Unlock()
		return toErrorDTO(fmt.Errorf("%w: providers: unknown provider %q", errNotFound, provider))
	}
	copyCfg, cleanup, err := cloneConfigForProviders(g.s.cfg)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		g.s.mu.Unlock()
		return toErrorDTO(err)
	}
	if copyCfg.Providers == nil {
		copyCfg.Providers = map[string]config.ProviderConfig{}
	}
	providerCfg := copyCfg.Providers[provider]
	accounts := make([]config.ProviderAccount, len(providerCfg.Accounts))
	copy(accounts, providerCfg.Accounts)
	found := false
	for i := range accounts {
		name := strings.TrimSpace(accounts[i].Name)
		if accounts[i].Ref == managedOAuthRef && name != accountName {
			g.s.mu.Unlock()
			return toErrorDTO(fmt.Errorf("%w: providers: managed credential already belongs to account %q", errConflict, name))
		}
		if name != accountName {
			continue
		}
		accounts[i] = config.ProviderAccount{Name: accountName, Kind: AccountKindToken, Ref: managedOAuthRef}
		found = true
	}
	if !found {
		accounts = append(accounts, config.ProviderAccount{
			Name: accountName,
			Kind: AccountKindToken,
			Ref:  managedOAuthRef,
		})
	}
	providerCfg.Accounts = accounts
	copyCfg.Providers[provider] = providerCfg

	previous, _, previousErr := store.Resolve(ctx, provider)
	if previousErr != nil && !errors.Is(previousErr, credential.ErrNotFound) {
		g.s.mu.Unlock()
		return toErrorDTO(previousErr)
	}
	if err := store.SaveAPIKey(provider, apiKey); err != nil {
		g.s.mu.Unlock()
		return toErrorDTO(err)
	}
	if err := g.s.Providers().persistConfigLocked(copyCfg); err != nil {
		if restoreErr := restoreManagedCredential(store, provider, previous, previousErr == nil); restoreErr != nil {
			err = fmt.Errorf("%w; managed credential restoration failed: %w", err, restoreErr)
		}
		g.s.mu.Unlock()
		return toErrorDTO(err)
	}
	g.s.mu.Unlock()
	g.s.emit(EventConfigChanged, map[string]string{"section": "providers"})
	g.s.emit(EventUsageUpdated, struct{}{})
	return nil
}

func restoreManagedCredential(store credential.ManagedStore, provider string, previous usage.Credential, found bool) error {
	if !found {
		err := store.Remove(provider)
		if errors.Is(err, credential.ErrNotFound) {
			return nil
		}
		return err
	}
	if previous.Source == usage.AuthEnvVar {
		return store.SaveAPIKey(provider, previous.Token)
	}
	return store.Save(provider, previous.Token)
}

func removeManagedCredential(
	ctx context.Context,
	store credential.ManagedStore,
	provider string,
) (func() error, error) {
	previous, _, err := store.Resolve(ctx, provider)
	if errors.Is(err, credential.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := store.Remove(provider); err != nil {
		return nil, err
	}
	return func() error {
		return restoreManagedCredential(store, provider, previous, true)
	}, nil
}

func (s *Services) managedStoreFor(_ string) (credential.ManagedStore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managedStoreLocked()
}

func (s *Services) managedStoreLocked() (credential.ManagedStore, error) {
	auth, err := s.cfg.LoadAuth()
	if err != nil {
		return credential.ManagedStore{}, err
	}
	return credential.ManagedStore{
		StateDir:    s.paths.StateDir,
		Keychain:    credential.DefaultKeychain(),
		UseKeychain: auth.UseKeychain,
	}, nil
}

func (g *SignInService) signInGate(provider string) error {
	g.s.mu.RLock()
	known := g.s.Providers().providerKnownLocked(provider)
	cfg := g.s.cfg
	g.s.mu.RUnlock()
	if !known {
		return fmt.Errorf("%w: unknown provider %q", errValidation, provider)
	}
	enabled, reason := toggle.ResolveUsageEnabled(false, cfg)
	if enabled {
		return nil
	}
	return fmt.Errorf("%w: usage is disabled (%s); enable usage before signing in", errUsageUnavailable, reason)
}

func providerOAuthSupported(provider string) bool {
	if provider == "claude" || provider == "codex" || provider == "antigravity" || provider == "cursor" {
		return true
	}
	desc, err := usage.Get(provider)
	if err != nil {
		return false
	}
	_, ok := deviceFlowSpec(desc)
	return ok
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
