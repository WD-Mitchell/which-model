//go:build !nousage

package credential

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

type MigrationOptions struct {
	RemoveSource bool
	Replace      bool
	// Transition persists a personal native-store preference after verification,
	// before any optional source removal. Nil means no transition is needed.
	Transition func() error
}

type MigrationReport struct {
	Provider     string `json:"provider"`
	SecureStore  string `json:"secure_store"`            // unchanged, unverified, verified
	LegacyCopy   string `json:"legacy_copy"`             // retained, removed, recovery
	RecoveryFile string `json:"recovery_file,omitempty"` // basename only; no identity
}

func migrationAuthority(provider string) error {
	p, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if err := p.RequireProvider(provider); err != nil {
		return err
	}
	if err := p.RequireSource("keychain"); err != nil {
		return err
	}
	return p.RequireCapability("credential_migration")
}

func decodeLegacy(data []byte) (usage.Credential, error) {
	var stored managedCredentialFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&stored) != nil {
		return usage.Credential{}, errors.New("legacy credential record is invalid")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return usage.Credential{}, errors.New("legacy credential record is invalid")
	}
	if stored.Source != "" && stored.Source != managedCredentialSourceAPIKey {
		return usage.Credential{}, errors.New("legacy credential source is unsupported")
	}
	if err := security.ValidateOpaqueToken(stored.Token); err != nil {
		return usage.Credential{}, err
	}
	return usage.Credential{Token: stored.Token, Source: managedCredentialSource(stored.Source), Extra: stored.Extra}, nil
}

func sameCredential(a, b usage.Credential) bool {
	normalize := func(c usage.Credential) map[string]string {
		out := map[string]string{}
		for key, value := range c.Extra {
			if key != "managed_store" {
				out[key] = value
			}
		}
		return out
	}
	return a.Token == b.Token && a.Source == b.Source && reflect.DeepEqual(normalize(a), normalize(b))
}

func (s ManagedStore) Migrate(ctx context.Context, provider string, opts MigrationOptions) (MigrationReport, error) {
	report := MigrationReport{Provider: provider, SecureStore: "unchanged", LegacyCopy: "retained"}
	if err := migrationAuthority(provider); err != nil {
		return report, err
	}
	path := s.Path(provider)
	if path == "" {
		return report, errors.New("legacy managed credential location is unavailable")
	}
	target := s
	target.UseKeychain = true
	target.NativeKeychain = true
	target.secureOnly = true
	target.keepLegacy = true
	if target.Keychain == nil {
		target.Keychain = KeychainFor(true)
	}
	return migrateFile(ctx, report, path, decodeLegacy,
		func(c usage.Credential) error { return target.SaveCredential(provider, c) },
		func() (usage.Credential, error) { c, _, err := target.Resolve(ctx, provider); return c, err }, opts)
}

// MigrateCatalog handles the separately owned legacy catalog key without reading
// provider-owned files or changing personal catalog-storage behavior.
func MigrateCatalog(ctx context.Context, configDir string, opts MigrationOptions) (MigrationReport, error) {
	report := MigrationReport{Provider: securestore.CatalogAccount, SecureStore: "unchanged", LegacyCopy: "retained"}
	if err := migrationAuthority(securestore.CatalogAccount); err != nil {
		return report, err
	}
	p, err := readCompanyPolicy()
	if err != nil {
		return report, err
	}
	if !p.Managed {
		return report, errors.New("catalog key migration requires the company profile; personal catalog storage is unchanged")
	}
	store := securestore.Native()
	return migrateFile(ctx, report, filepath.Join(configDir, securestore.CatalogLegacyFile),
		func(data []byte) (usage.Credential, error) {
			token := strings.TrimSpace(string(data))
			if err := security.ValidateOpaqueToken(token); err != nil {
				return usage.Credential{}, err
			}
			return usage.Credential{Token: token, Source: usage.AuthEnvVar}, nil
		},
		func(c usage.Credential) error {
			return store.Set(securestore.CatalogService, securestore.CatalogAccount, c.Token)
		},
		func() (usage.Credential, error) {
			token, err := store.Get(securestore.CatalogService, securestore.CatalogAccount)
			if err != nil {
				return usage.Credential{}, nativeStoreFailure(err)
			}
			return usage.Credential{Token: token, Source: usage.AuthEnvVar}, nil
		}, opts)
}

func migrateFile(ctx context.Context, report MigrationReport, path string, decode func([]byte) (usage.Credential, error), save func(usage.Credential) error, resolve func() (usage.Credential, error), opts MigrationOptions) (MigrationReport, error) {
	if err := ctx.Err(); err != nil {
		return report, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 64*1024 {
		return report, errors.New("legacy credential file is missing, redirected or unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return report, errors.New("legacy credential file could not be read")
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		file.Close()
		return report, errors.New("legacy credential file changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, 64*1024+1))
	file.Close()
	if err != nil || len(data) > 64*1024 {
		return report, errors.New("legacy credential file could not be read safely")
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, current) || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return report, errors.New("legacy credential file changed")
	}
	candidate, err := decode(data)
	if err != nil {
		return report, err
	}
	previous, err := resolve()
	if err != nil && !errors.Is(err, ErrNotFound) {
		return report, err
	}
	if err == nil && !sameCredential(previous, candidate) && !opts.Replace {
		return report, errors.New("OS store contains a different credential; use --replace only to replace that owned entry")
	}
	report.SecureStore = "unverified"
	if err := save(candidate); err != nil {
		return report, err
	}
	verified, err := resolve()
	if err != nil || !sameCredential(verified, candidate) {
		return report, errors.New("secure credential could not be verified; legacy copy retained")
	}
	report.SecureStore = "verified"
	if opts.Transition != nil {
		if err := opts.Transition(); err != nil {
			return report, errors.New("secure credential verified but configuration transition failed; legacy copy retained")
		}
	}
	if !opts.RemoveSource {
		return report, nil
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	// Atomically move the selected source before testing/deleting it. A concurrent
	// replacement is retained/restored rather than deleting a newer credential.
	staging, err := os.CreateTemp(filepath.Dir(path), ".which-model-migration-*")
	if err != nil {
		return report, errors.New("secure credential verified; legacy copy could not be removed")
	}
	quarantine := staging.Name()
	staging.Close()
	if err := os.Rename(path, quarantine); err != nil {
		os.Remove(quarantine)
		return report, errors.New("secure credential verified; legacy copy could not be removed")
	}
	moved, readErr := readSameMigrationFile(quarantine, info)
	if readErr != nil || sha256.Sum256(moved) != sha256.Sum256(data) {
		if os.Link(quarantine, path) == nil {
			if os.Remove(quarantine) != nil {
				report.LegacyCopy = "recovery"
				report.RecoveryFile = filepath.Base(quarantine)
			}
		} else {
			report.LegacyCopy = "recovery"
			report.RecoveryFile = filepath.Base(quarantine)
		}
		return report, errors.New("source changed during migration; the changed copy was retained")
	}
	if err := os.Remove(quarantine); err != nil {
		report.LegacyCopy = "recovery"
		report.RecoveryFile = filepath.Base(quarantine)
		return report, errors.New("secure credential verified; legacy recovery copy could not be removed")
	}
	report.LegacyCopy = "removed"
	return report, nil
}

func readSameMigrationFile(path string, expected os.FileInfo) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, info) || info.Size() > 64*1024 {
		return nil, errors.New("source changed")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(expected, opened) {
		return nil, errors.New("source changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(data) > 64*1024 {
		return nil, errors.New("source changed")
	}
	return data, nil
}
