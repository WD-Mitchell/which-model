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
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const managedKeychainService = "which-model"

const managedCredentialSourceAPIKey = "api_key"

var managedProviderPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type managedCredentialFile struct {
	Token  string `json:"token"`
	Source string `json:"source,omitempty"`
}

// ManagedStore persists credentials created by which-model. The OS keychain
// is preferred when enabled; its absence degrades to a private state file.
type ManagedStore struct {
	StateDir    string
	Keychain    ManagedKeychainStore
	UseKeychain bool
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
	return DefaultKeychain()
}

// Save validates and persists an OAuth token without including credential
// material in any error. A keychain write failure falls back to the state file.
func (s ManagedStore) Save(provider, token string) error {
	return s.save(provider, token, "")
}

// SaveAPIKey stores an API key with source metadata so resolution never treats
// it as an OAuth credential or runs an OAuth-token validator against it.
func (s ManagedStore) SaveAPIKey(provider, token string) error {
	return s.save(provider, token, managedCredentialSourceAPIKey)
}

func (s ManagedStore) save(provider, token, source string) error {
	path := s.Path(provider)
	if path == "" {
		return errors.New("managed credential storage is unavailable")
	}
	if err := security.ValidateOpaqueToken(token); err != nil {
		return errors.New("credential has an unsafe value")
	}
	stored := managedCredentialFile{Token: token, Source: source}
	data, err := json.Marshal(stored)
	if err != nil {
		return errors.New("managed credential encoding failed")
	}
	keychainValue := token
	if source != "" {
		keychainValue = string(data)
	}
	if s.UseKeychain {
		if err := s.keychain().Set(managedKeychainService, provider, keychainValue); err == nil {
			// The credential is committed once Keychain accepts it. Fallback
			// cleanup is best-effort: reporting a failure here would tell the
			// caller to retry after authentication state had already changed.
			_ = os.Remove(path)
			return nil
		}
	}
	if err := config.AtomicWriteFile(path, append(data, '\n')); err != nil {
		return errors.New("managed credential file write failed")
	}
	return nil
}

// Resolve loads a managed credential. Keychain failures are deliberately
// treated as unavailable so a fallback file remains usable.
func (s ManagedStore) Resolve(ctx context.Context, provider string) (usage.Credential, []Warning, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, nil, err
	}
	path := s.Path(provider)
	if path == "" {
		return Credential{}, nil, ErrNotFound
	}
	var warnings []Warning
	if s.UseKeychain {
		value, err := s.keychain().Get(managedKeychainService, provider)
		if err == nil && value != "" {
			stored := managedCredentialFile{Token: value}
			var encoded managedCredentialFile
			if json.Unmarshal([]byte(value), &encoded) == nil && encoded.Token != "" {
				stored = encoded
			}
			if security.ValidateOpaqueToken(stored.Token) == nil {
				return Credential{Token: stored.Token, Source: managedCredentialSource(stored.Source)}, nil, nil
			}
		} else if err != nil && !errors.Is(err, keyringNotFound) && !errors.Is(err, ErrNotFound) {
			warnings = append(warnings, Warning{Message: "system keychain unavailable; using managed credential file"})
		}
	}

	info, err := os.Stat(path)
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
	data, err := os.ReadFile(path)
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
	return Credential{Token: stored.Token, Source: managedCredentialSource(stored.Source)}, warnings, nil
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
	path := s.Path(provider)
	if path == "" {
		return ErrNotFound
	}
	found := false
	keychainErr := s.keychain().Delete(managedKeychainService, provider)
	if keychainErr == nil {
		found = true
	} else if !errors.Is(keychainErr, keyringNotFound) && !errors.Is(keychainErr, ErrNotFound) {
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

// ResolveProvider preserves provider-declared source precedence, then tries a
// credential saved by interactive Settings/CLI login (ManagedStore). Device-flow
// providers may still Validate that token; file-only providers (Claude, Codex)
// use the store as a last source after declared files/env miss.
func ResolveProvider(ctx context.Context, provider string, sources []usage.AuthSource, client *http.Client, store ManagedStore) (usage.Credential, []Warning, error) {
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
