//go:build !nousage

package credential

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const managedKeychainService = "which-model"

const managedCredentialSourceAPIKey = "api_key"

var managedProviderPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type managedCredentialFile struct {
	Token  string            `json:"token"`
	Source string            `json:"source,omitempty"`
	Extra  map[string]string `json:"extra,omitempty"`
}

// ManagedStore persists credentials created by which-model. The OS keychain
// is preferred when enabled. Personal legacy mode may use a private state file;
// company secure-store-only mode never permits that fallback.
type ManagedStore struct {
	StateDir       string
	Keychain       ManagedKeychainStore
	UseKeychain    bool
	NativeKeychain bool
	secureOnly     bool
	keepLegacy     bool
}

// Path returns the fallback credential path, resolving the platform state
// directory when StateDir is empty. Invalid provider identifiers or an
// unavailable home directory return an empty string.
func (s ManagedStore) Path(provider string) string {
	if !managedProviderPattern.MatchString(provider) {
		return ""
	}
	stateDir := s.StateDir
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = config.ResolvePaths(runtime.GOOS, home, os.Getenv).StateDir
	}
	return filepath.Join(stateDir, "credentials", provider+".json")
}

func (s ManagedStore) keychain() ManagedKeychainStore {
	if s.Keychain != nil {
		return s.Keychain
	}
	return KeychainFor(s.NativeKeychain)
}

// Save validates and persists an OAuth token without including credential
// material in any error. Personal legacy mode may fall back to the state file.
func (s ManagedStore) Save(provider, token string) error {
	return s.save(provider, token, "")
}

// SaveAPIKey stores an API key with source metadata so resolution never treats
// it as an OAuth credential or runs an OAuth-token validator against it.
func (s ManagedStore) SaveAPIKey(provider, token string) error {
	return s.save(provider, token, managedCredentialSourceAPIKey)
}

func (s ManagedStore) save(provider, token, source string) error {
	return s.saveRecord(provider, managedCredentialFile{Token: token, Source: source})
}

// SaveCredential preserves only metadata required by native usage adapters.
func (s ManagedStore) SaveCredential(provider string, cred usage.Credential) error {
	source := ""
	if cred.Source == usage.AuthEnvVar {
		source = managedCredentialSourceAPIKey
	}
	extra := make(map[string]string, len(cred.Extra))
	for key, value := range cred.Extra {
		if key == "managed_store" {
			continue
		}
		if key != "account_id" && key != "expires_at" {
			return errors.New("unsupported managed credential metadata")
		}
		if len(value) > 1024 {
			return errors.New("invalid managed credential metadata")
		}
		extra[key] = value
	}
	return s.saveRecord(provider, managedCredentialFile{Token: cred.Token, Source: source, Extra: extra})
}

func (s ManagedStore) saveRecord(provider string, stored managedCredentialFile) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if err := policy.RequireProvider(provider); err != nil {
		return err
	}
	path := s.Path(provider)
	if path == "" {
		return errors.New("managed credential storage is unavailable")
	}
	if err := security.ValidateOpaqueToken(stored.Token); err != nil {
		return errors.New("credential has an unsafe value")
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return errors.New("managed credential encoding failed")
	}
	keychainValue := stored.Token
	if stored.Source != "" || len(stored.Extra) > 0 {
		keychainValue = string(data)
	}
	if s.UseKeychain {
		if err := policy.RequireSource("keychain"); err != nil {
			return err
		}
		if err := s.keychain().Set(managedKeychainService, provider, keychainValue); err == nil {
			// The credential is committed once Keychain accepts it. Fallback
			// cleanup is best-effort: reporting a failure here would tell the
			// caller to retry after authentication state had already changed.
			// Managed enrollment does not authorize silent legacy-file deletion.
			if !policy.Managed && !s.keepLegacy {
				_ = os.Remove(path)
			}
			return nil
		} else if s.secureOnly || (policy.Managed && policy.Policy.SecureStoreOnly) {
			return nativeStoreFailure(err)
		}
	}
	if err := policy.RequireSource("managed_file"); err != nil {
		return err
	}
	if err := managedFileWrite(path, append(data, '\n')); err != nil {
		return errors.New("managed credential file write failed")
	}
	return nil
}

// Resolve loads a managed credential. Keychain failures are deliberately
// mapped to fixed failures in company secure-store-only mode. Personal legacy
// mode retains its fallback behavior.
func (s ManagedStore) Resolve(ctx context.Context, provider string) (usage.Credential, []Warning, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, nil, err
	}
	policy, err := readCompanyPolicy()
	if err != nil {
		return Credential{}, nil, err
	}
	if err := policy.RequireProvider(provider); err != nil {
		return Credential{}, nil, err
	}
	path := s.Path(provider)
	if path == "" {
		return Credential{}, nil, ErrNotFound
	}
	var warnings []Warning
	if s.UseKeychain {
		if err := policy.RequireSource("keychain"); err != nil {
			return Credential{}, nil, err
		}
		value, err := managedKeychainGet(ctx, s.keychain(), managedKeychainService, provider)
		if ctx.Err() != nil {
			return Credential{}, nil, ctx.Err()
		}
		if err == nil && value != "" {
			stored := managedCredentialFile{Token: value}
			var encoded managedCredentialFile
			if json.Unmarshal([]byte(value), &encoded) == nil && encoded.Token != "" {
				stored = encoded
			}
			if security.ValidateOpaqueToken(stored.Token) == nil {
				resolvedStore := ""
				if policy.Managed || s.NativeKeychain || s.secureOnly {
					resolvedStore = "keychain"
				}
				return resolvedManagedCredential(stored, resolvedStore), nil, nil
			}
			if s.secureOnly || (policy.Managed && policy.Policy.SecureStoreOnly) {
				return Credential{}, nil, usage.NewFailureError("unsafe_credential", "OS secure store contains an invalid credential")
			}
		} else if s.secureOnly || (policy.Managed && policy.Policy.SecureStoreOnly) {
			if err == nil {
				return Credential{}, nil, ErrNotFound
			}
			return Credential{}, nil, nativeStoreFailure(err)
		} else if err != nil && !errors.Is(err, keyringNotFound) && !errors.Is(err, ErrNotFound) && !errors.Is(err, &securestore.Error{Kind: securestore.Missing}) {
			if !policy.Managed {
				warnings = append(warnings, Warning{Message: "system keychain unavailable; using managed credential file"})
			}
		}
	}

	if err := policy.RequireSource("managed_file"); err != nil {
		return Credential{}, warnings, err
	}
	info, err := managedFileStat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credential{}, warnings, ErrNotFound
		}
		return Credential{}, warnings, usage.NewFailureError("credential_file", "managed credential file could not be read")
	}
	if info.Size() > 64*1024 {
		return Credential{}, warnings, usage.NewFailureError("credential_file", "managed credential file is too large")
	}
	if info.Mode().Perm()&0o077 != 0 {
		warnings = append(warnings, Warning{Message: fmt.Sprintf("credential file %q has broad permissions", path)})
	}
	data, err := managedFileRead(path)
	if err != nil {
		return Credential{}, warnings, usage.NewFailureError("credential_file", "managed credential file could not be read")
	}
	var stored managedCredentialFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return Credential{}, warnings, usage.NewFailureError("credential_json", "managed credential file contains invalid JSON")
	}
	if err := security.ValidateOpaqueToken(stored.Token); err != nil {
		return Credential{}, warnings, usage.NewFailureError("unsafe_credential", "managed credential file contains an unsafe credential")
	}
	resolvedStore := ""
	if policy.Managed || s.NativeKeychain {
		resolvedStore = "managed_file"
	}
	return resolvedManagedCredential(stored, resolvedStore), warnings, nil
}

func resolvedManagedCredential(stored managedCredentialFile, source string) Credential {
	extra := make(map[string]string, len(stored.Extra)+1)
	for key, value := range stored.Extra {
		if key != "managed_store" {
			extra[key] = value
		}
	}
	// Provenance describes this resolution, never a claim read from the record.
	if source != "" {
		extra["managed_store"] = source
	}
	return Credential{Token: stored.Token, Source: managedCredentialSource(stored.Source), Extra: extra}
}

func managedCredentialSource(source string) usage.AuthKind {
	if source == managedCredentialSourceAPIKey {
		return usage.AuthEnvVar
	}
	return usage.AuthOAuthDeviceFlow
}

// Remove deletes both managed locations so toggling keychain use cannot leave
// a credential active in an older store.
func (s ManagedStore) Remove(provider string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if policy.Managed {
		return s.RemoveSecure(provider)
	}
	path := s.Path(provider)
	if path == "" {
		return ErrNotFound
	}
	found := false
	keychainErr := s.keychain().Delete(managedKeychainService, provider)
	if keychainErr == nil {
		found = true
	} else if !errors.Is(keychainErr, keyringNotFound) && !errors.Is(keychainErr, ErrNotFound) && !errors.Is(keychainErr, &securestore.Error{Kind: securestore.Missing}) {
		keychainErr = errors.New("managed credential keychain removal failed")
	} else {
		keychainErr = nil
	}
	if err := os.Remove(path); err == nil {
		found = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("managed credential file removal failed")
	}
	if keychainErr != nil {
		return keychainErr
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

// RemoveSecure removes only the app-owned OS item. It performs no legacy-file
// probe and is also the safe rollback for a newly created secure credential.
func (s ManagedStore) RemoveSecure(provider string) error {
	if _, err := readCompanyPolicy(); err != nil {
		return err
	}
	if !managedProviderPattern.MatchString(provider) {
		return ErrNotFound
	}
	err := s.keychain().Delete(managedKeychainService, provider)
	if err == nil {
		return nil
	}
	return nativeStoreFailure(err)
}

// ResolveProvider preserves provider-declared source precedence, then tries a
// credential saved by interactive Settings/CLI login (ManagedStore). Device-flow
// providers may still Validate that token; file-only providers (Claude, Codex)
// use the store as a last source after declared files/env miss.
func ResolveProvider(ctx context.Context, provider string, sources []usage.AuthSource, client *http.Client, store ManagedStore) (usage.Credential, []Warning, error) {
	policy, policyErr := readCompanyPolicy()
	if policyErr != nil {
		return Credential{}, nil, policyErr
	}
	if err := policy.RequireProvider(provider); err != nil {
		return Credential{}, nil, err
	}
	credential, warnings, err := ResolveChain(ctx, sources, client)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return credential, warnings, err
	}
	credential, managedWarnings, managedErr := store.Resolve(ctx, provider)
	warnings = append(warnings, managedWarnings...)
	if managedErr != nil {
		return Credential{}, warnings, managedErr
	}
	if credential.Source == usage.AuthOAuthDeviceFlow {
		for _, source := range sources {
			if source.Kind != usage.AuthOAuthDeviceFlow || source.Validate == nil {
				continue
			}
			if err := source.Validate(ctx, credential, client); err != nil {
				return Credential{}, warnings, ErrNotFound
			}
			break
		}
	}
	return credential, warnings, nil
}

// The keychain interface cannot cancel an OS prompt. Bound both caller latency
// and outstanding noncancellable calls; a late result is discarded securely.
var managedLookupSlots = make(chan struct{}, 4)

func managedKeychainGet(ctx context.Context, store ManagedKeychainStore, service, account string) (string, error) {
	select {
	case managedLookupSlots <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-managedLookupSlots }()
		value, err := store.Get(service, account)
		done <- result{value, err}
	}()
	select {
	case r := <-done:
		return r.value, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
