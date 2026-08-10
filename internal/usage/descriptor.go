//go:build !nousage

package usage

import (
	"context"
	"net/http"
	"time"
)

type KeychainSpec struct {
	Service string // e.g. "Claude Code-credentials"
	Account string // "" = match any account for the service
	Server  string // internet-password only, e.g. "zed.dev"
}

type CookieSpec struct {
	Domains     []string // exact cookie-jar domains to query, never a wildcard
	CookieNames []string // strict-name pass first; empty = accept any non-empty set for Domains
}

type ShellSpec struct {
	Command string
	Args    []string
	Timeout time.Duration
}

type RPCSpec struct {
	Command     string
	Args        []string
	InitTimeout time.Duration
	CallTimeout time.Duration
	Method      string // JSON-RPC method invoked after initialize, e.g. "account/rateLimits/read"
}

type OAuthSpec struct {
	ClientID        string
	ClientSecret    string // empty for public clients (Codex, Claude, Copilot)
	DeviceCodeURL   string // device flow only
	TokenURL        string
	Scope           string
	VerificationURI string // device flow: EXACT match for the server-provided verification_uri (F12)
}

// AuthSource is one ordered link in a provider's credential-resolution chain.
// Descriptor.Auth is walked in order (F12 ResolveChain); the first entry that
// both resolves a candidate AND (when Validate is set) passes validation wins.
type AuthSource struct {
	Kind AuthKind

	EnvVar     string   // AuthEnvVar
	FilePaths  []string // AuthFile: ordered candidate paths, first existing+valid wins
	JSONPath   string   // AuthFile: dotted path to the token field, e.g. "tokens.access_token"
	ExpiryPath string   // AuthFile: dotted path to the expiry epoch/date; "" = no expiry check

	Keychain *KeychainSpec // AuthKeychainGeneric / AuthKeychainInternet
	Cookie   *CookieSpec   // AuthBrowserCookie
	Shell    *ShellSpec    // AuthCLIShellOut
	RPC      *RPCSpec      // AuthSubprocessRPC
	OAuth    *OAuthSpec    // AuthOAuthDeviceFlow / AuthOAuthRefreshGrant

	// ExtraPaths: Extra-field name → dotted JSON path inside the credential
	// file, e.g. {"account_id": "tokens.account_id", "base_url": "flat base_url"}.
	ExtraPaths map[string]string
	// EnvExtra: extra env var names copied into Credential.Extra when set
	// (e.g. "OPENAI_PROJECT_ID").
	EnvExtra []string

	// Validate gates candidate acceptance (e.g. Copilot's mandatory GET /user
	// identity check). A candidate that fails Validate is skipped, not fatal.
	Validate func(ctx context.Context, candidate Credential, client *http.Client) error
}

// FetchFunc performs one provider's usage fetch given a resolved credential.
// Provider packages leave Snapshot.Source unset — the fetch layer (F14) owns
// Source population (see specs/features/F14-usage-fetch/CONTRACTS.md §3).
type FetchFunc func(ctx context.Context, cred Credential, client *http.Client) (Snapshot, error)

// Descriptor is exactly the contract type for one provider adapter.
type Descriptor struct {
	ID           string
	DisplayName  string
	Kind         Kind
	Tier         int           // 1 = port first, 2 = second wave, 3 = deferred
	Auth         []AuthSource
	Windows      []WindowSpec
	Timeout      time.Duration // per-fetch timeout; 0 = use global DefaultTimeoutSec (10s)
	CacheTTL     time.Duration // usage cache TTL; 0 = this provider is never cached (F13/F14)
	Fetch        FetchFunc
	LastVerified time.Time // zero = never verified; populated by F25 auth flows
}
