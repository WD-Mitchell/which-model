//go:build !nousage

package service

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

type AdministrationService struct{ s *Services }

func (s *Services) Administration() *AdministrationService { return &AdministrationService{s: s} }

type MigrationResult struct {
	credential.MigrationReport
	Error string `json:"error,omitempty"`
}

var desktopPrivacyLayout = privacy.DefaultLayout
var migrateDesktopCredential = func(ctx context.Context, store credential.ManagedStore, provider string, opts credential.MigrationOptions) (credential.MigrationReport, error) {
	return store.Migrate(ctx, provider, opts)
}

func (a *AdministrationService) Status(ctx context.Context) (AdministrationStatus, error) {
	if err := ctx.Err(); err != nil {
		return AdministrationStatus{}, err
	}
	policy, err := readCompanyPolicy()
	if err != nil {
		return AdministrationStatus{}, err
	}
	a.s.mu.RLock()
	auth, err := a.s.cfg.LoadAuth()
	a.s.mu.RUnlock()
	if err != nil {
		return AdministrationStatus{}, err
	}
	a.s.privacyMu.Lock()
	defer a.s.privacyMu.Unlock()
	var last *MaintenanceResult
	if a.s.lastMaintenance != nil {
		copy := *a.s.lastMaintenance
		copy.Categories = make(map[string]privacy.Report, len(copy.Categories))
		for k, v := range a.s.lastMaintenance.Categories {
			copy.Categories[k] = v
		}
		last = &copy
	}
	return AdministrationStatus{Policy: policy, NativeKeychain: policy.Managed || auth.NativeKeychain, UseKeychain: policy.Managed || auth.UseKeychain, LastMaintenance: last}, nil
}

func (a *AdministrationService) SetNativeStore(ctx context.Context, enabled bool) error {
	a.s.credentialMu.Lock()
	defer a.s.credentialMu.Unlock()
	return a.setNativeStore(ctx, enabled)
}

// setNativeStore is also the migration transition while credentialMu is held.
func (a *AdministrationService) setNativeStore(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if policy.Managed {
		return &company.Error{Reason: "native credential storage is administrator-controlled"}
	}
	a.s.mu.Lock()
	next := a.s.cfg.Clone()
	auth, err := next.LoadAuth()
	if err == nil {
		auth.NativeKeychain = enabled
		if enabled {
			auth.UseKeychain = true
		}
		err = next.SetAuth(auth)
	}
	if err == nil {
		var data []byte
		data, err = next.MarshalTOML()
		if err == nil {
			err = config.AtomicWriteFile(a.s.paths.UserConfigFile, data)
		}
	}
	if err == nil {
		a.s.cfg = next
	}
	a.s.mu.Unlock()
	if err == nil {
		a.s.emit(EventConfigChanged, map[string]string{"section": "auth"})
		settings, e := a.s.Settings().Get(ctx)
		if e == nil {
			a.s.emit(EventSettingsChanged, settings)
		}
	}
	return err
}

func (a *AdministrationService) Migrate(ctx context.Context, provider string, removeSource, replace bool) (MigrationResult, error) {
	if err := ctx.Err(); err != nil {
		return MigrationResult{}, err
	}
	a.s.credentialMu.Lock()
	defer a.s.credentialMu.Unlock()
	policy, err := readCompanyPolicy()
	if err != nil {
		return MigrationResult{}, err
	}
	// Validate before any store or filesystem access. The migration implementation
	// independently rechecks protected authority immediately before doing its work.
	a.s.mu.RLock()
	known := a.s.Providers().providerKnownLocked(provider)
	a.s.mu.RUnlock()
	if !known && provider != securestore.CatalogAccount {
		return MigrationResult{}, fmt.Errorf("%w: unknown migration provider", errValidation)
	}
	if err := policy.RequireProvider(provider); err != nil {
		return MigrationResult{}, err
	}
	if err := policy.RequireSource("keychain"); err != nil {
		return MigrationResult{}, err
	}
	if err := policy.RequireCapability("credential_migration"); err != nil {
		return MigrationResult{}, err
	}
	opts := credential.MigrationOptions{RemoveSource: removeSource, Replace: replace}
	if !policy.Managed {
		opts.Transition = func() error { return a.setNativeStore(ctx, true) }
	}
	var report credential.MigrationReport
	if provider == securestore.CatalogAccount {
		report, err = credential.MigrateCatalog(ctx, a.s.paths.ConfigDir, opts)
	} else {
		store, e := a.s.managedStoreFor(provider)
		if e != nil {
			return MigrationResult{}, e
		}
		store.Keychain = credential.KeychainFor(true)
		report, err = migrateDesktopCredential(ctx, store, provider, opts)
	}
	result := MigrationResult{MigrationReport: report}
	if err != nil {
		result.Error = err.Error()
	}
	a.s.emit(EventConfigChanged, map[string]string{"section": "auth"})
	return result, nil
}

func (s *Services) privacyLayout(projectRoot string) (privacy.Layout, error) {
	layout, err := desktopPrivacyLayout()
	if err != nil {
		return layout, err
	}
	layout.StateDir = s.paths.StateDir
	layout.CacheDirs = append(layout.CacheDirs, filepath.Join(s.paths.CacheDir, "usage-cache"))
	if s.usageCacheDir != "" {
		layout.CacheDirs = append(layout.CacheDirs, s.usageCacheDir)
	}
	if projectRoot != "" {
		layout.ProjectRoot = projectRoot
	}
	return layout, nil
}

func (a *AdministrationService) Maintain(ctx context.Context, operation string, selected []string, projectRoot string, confirmed bool) (MaintenanceResult, error) {
	if err := ctx.Err(); err != nil {
		return MaintenanceResult{}, err
	}
	if operation != "cleanup" && operation != "purge" {
		return MaintenanceResult{}, fmt.Errorf("%w: unknown maintenance operation", errValidation)
	}
	if operation == "purge" && (!confirmed || len(selected) == 0) {
		return MaintenanceResult{}, fmt.Errorf("%w: select categories and confirm deletion", errValidation)
	}
	if projectRoot != "" && (!filepath.IsAbs(projectRoot) || filepath.Clean(projectRoot) != projectRoot) {
		return MaintenanceResult{}, fmt.Errorf("%w: project root must be an absolute directory", errValidation)
	}
	categories := []privacy.Category{}
	for _, s := range selected {
		c := privacy.Category(s)
		if !slices.Contains([]privacy.Category{privacy.Usage, privacy.History, privacy.Audit, privacy.Launch}, c) {
			return MaintenanceResult{}, fmt.Errorf("%w: invalid privacy category", errValidation)
		}
		if !slices.Contains(categories, c) {
			categories = append(categories, c)
		}
	}
	policy, err := readCompanyPolicy()
	if err != nil {
		return MaintenanceResult{}, err
	}
	ctl := privacy.Controller{Policy: policy}
	if operation == "purge" && !ctl.Enabled() {
		p := company.Defaults()
		ctl.Policy = company.Snapshot{Managed: true, Policy: &p}
	}
	layout, err := a.s.privacyLayout(projectRoot)
	if err != nil {
		return MaintenanceResult{}, err
	}
	// Do not race an in-flight usage fetch that could repopulate purged evidence.
	a.s.usageFetchMu.Lock()
	result := a.s.maintainWithReport(ctl, layout, operation, categories)
	result.Managed = policy.Managed
	if operation == "purge" && slices.Contains(categories, privacy.Usage) {
		a.s.mu.Lock()
		a.s.usageObservations = nil
		a.s.usageObservationMaxAge = nil
		a.s.mu.Unlock()
	}
	a.s.usageFetchMu.Unlock()
	a.s.privacyMu.Lock()
	a.s.lastMaintenance = &result
	a.s.privacyMu.Unlock()
	a.s.emit(EventConfigChanged, map[string]string{"section": "privacy"})
	return result, nil
}

func (s *Services) maintainWithReport(ctl privacy.Controller, layout privacy.Layout, operation string, categories []privacy.Category) MaintenanceResult {
	s.privacyMu.Lock()
	defer s.privacyMu.Unlock()
	summary, err := ctl.Maintain(layout, operation == "purge", categories)
	result := MaintenanceResult{Managed: summary.Managed, Categories: map[string]privacy.Report{}, Operation: operation, CompletedAt: time.Now().UTC().Format(time.RFC3339)}
	for key, report := range summary.Categories {
		result.Categories[string(key)] = report
	}
	if err != nil {
		result.Error = err.Error()
	}
	s.lastMaintenance = &result
	return result
}

func (a *AdministrationService) VerifyDelegation(ctx context.Context) ([]DelegationCheck, error) {
	policy, err := readCompanyPolicy()
	if err != nil {
		return nil, err
	}
	results := []DelegationCheck{}
	if !policy.Managed || policy.Policy == nil {
		return results, nil
	}
	for _, entry := range policy.Policy.CodexBarInstallations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := DelegationCheck{Path: entry.Path, ConfigPath: entry.Config.Path}
		err := company.VerifyInstallation(company.Installation{Path: entry.Path, SHA256: entry.SHA256}, true)
		if err == nil {
			err = company.VerifyInstallation(entry.Config, false)
		}
		r.Verified = err == nil
		if err != nil {
			r.Error = "Approved image or configuration could not be verified; deployment needs administrator attention."
		}
		results = append(results, r)
	}
	return results, nil
}
