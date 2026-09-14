//go:build !nousage

package service

import (
	"context"
	"errors"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditPolicy(t *testing.T, managed bool) {
	t.Helper()
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	p.AllowCredentialMigration = true
	p.Executables = []company.Executable{{ID: "codex", Path: "/approved/codex", Args: []string{"-m", "{model_id}", "-c", "model_reasoning_effort=high"}}}
	readCompanyPolicy = func() (company.Snapshot, error) {
		if !managed {
			return company.Snapshot{}, nil
		}
		return company.Snapshot{Managed: true, Policy: &p}, nil
	}
}
func TestDesktopManagedPreviewUsesProtectedTemplate(t *testing.T) {
	s, _ := newTestServices(t, WithConfigTOML("[harnesses.codex]\nname='Codex'\ncommand='UNUSED_USER_COMMAND --model {model_id}'\nbuiltin=true\n"))
	auditPolicy(t, true)
	rows, err := s.Harnesses().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range rows {
		if h.Slug == "codex" {
			if !h.CommandManaged || !h.CommandAvailable || strings.Contains(h.Command, "UNUSED") || !strings.Contains(h.Command, "/approved/codex") || !strings.Contains(h.Command, "model_reasoning_effort=high") {
				t.Fatalf("incorrect protected preview: %+v", h)
			}
			return
		}
	}
	t.Fatal("codex missing")
}
func TestDesktopAdministrationPolicyAndPreference(t *testing.T) {
	s, _ := newTestServices(t)
	auditPolicy(t, true)
	status, err := s.Administration().Status(context.Background())
	if err != nil || !status.Policy.Managed || !status.NativeKeychain {
		t.Fatalf("%+v %v", status, err)
	}
	if err := s.Administration().SetNativeStore(context.Background(), false); err == nil {
		t.Fatal("managed preference changed")
	}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{}, nil }
	if err := s.Administration().SetNativeStore(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	auth, _ := s.cfg.LoadAuth()
	if !auth.NativeKeychain || !auth.UseKeychain {
		t.Fatal("native store not enabled")
	}
}
func TestDesktopMigrationKeepsPartialReport(t *testing.T) {
	s, _ := newTestServices(t)
	auditPolicy(t, false)
	old := migrateDesktopCredential
	t.Cleanup(func() { migrateDesktopCredential = old })
	migrateDesktopCredential = func(ctx context.Context, store credential.ManagedStore, id string, o credential.MigrationOptions) (credential.MigrationReport, error) {
		if !o.RemoveSource || o.Replace {
			t.Fatal("wrong explicit options")
		}
		if err := o.Transition(); err != nil {
			t.Fatal(err)
		}
		return credential.MigrationReport{Provider: id, SecureStore: "verified", LegacyCopy: "recovery", RecoveryFile: ".which-model-migration-test"}, errors.New("source removal incomplete")
	}
	r, err := s.Administration().Migrate(context.Background(), "codex", true, false)
	if err != nil || r.SecureStore != "verified" || r.LegacyCopy != "recovery" || r.Error == "" {
		t.Fatalf("partial result lost: %+v %v", r, err)
	}
	auth, _ := s.cfg.LoadAuth()
	if !auth.NativeKeychain {
		t.Fatal("live config not transitioned")
	}
}
func TestDesktopPrivacyRequiresConfirmationAndPreservesOtherCategories(t *testing.T) {
	s, _ := newTestServices(t)
	auditPolicy(t, false)
	old := desktopPrivacyLayout
	t.Cleanup(func() { desktopPrivacyLayout = old })
	desktopPrivacyLayout = func() (privacy.Layout, error) { return privacy.Layout{StateDir: s.paths.StateDir}, nil }
	dir := filepath.Join(s.paths.StateDir, "pick")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, []byte("{}\n"), 0600)
	a := s.Administration()
	if _, err := a.Maintain(context.Background(), "purge", []string{"pick_history"}, "", false); err == nil {
		t.Fatal("purge without confirmation")
	}
	if _, err := a.Maintain(context.Background(), "purge", nil, "", true); err == nil {
		t.Fatal("empty category purge")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("unconfirmed purge touched history")
	}
	r, err := a.Maintain(context.Background(), "purge", []string{"launch_logs"}, "", true)
	if err != nil || r.Error != "" {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("purge touched unselected history")
	}
	r, err = a.Maintain(context.Background(), "purge", []string{"pick_history"}, "", true)
	if err != nil || r.Error != "" {
		t.Fatalf("%+v %v", r, err)
	}
	if b, _ := os.ReadFile(path); len(b) != 0 {
		t.Fatal("selected history survived")
	}
	status, _ := a.Status(context.Background())
	if status.LastMaintenance == nil || status.LastMaintenance.Operation != "purge" {
		t.Fatal("last result missing")
	}
}

func TestDesktopMigrationDenialPrecedesStoreAccess(t *testing.T) {
	s, _ := newTestServices(t)
	auditPolicy(t, true)
	p := company.Defaults()
	p.AllowedProviders = []string{"codex"}
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	old := migrateDesktopCredential
	t.Cleanup(func() { migrateDesktopCredential = old })
	migrateDesktopCredential = func(context.Context, credential.ManagedStore, string, credential.MigrationOptions) (credential.MigrationReport, error) {
		t.Fatal("denied migration touched store")
		return credential.MigrationReport{}, nil
	}
	if _, err := s.Administration().Migrate(context.Background(), "codex", true, true); err == nil {
		t.Fatal("migration permission bypassed")
	}
}
func TestDesktopPrivacyReportsPartialFailure(t *testing.T) {
	s, _ := newTestServices(t)
	auditPolicy(t, false)
	old := desktopPrivacyLayout
	t.Cleanup(func() { desktopPrivacyLayout = old })
	desktopPrivacyLayout = func() (privacy.Layout, error) { return privacy.Layout{StateDir: s.paths.StateDir}, nil }
	dir := filepath.Join(s.paths.StateDir, "pick")
	os.MkdirAll(dir, 0700)
	foreign := filepath.Join(t.TempDir(), "foreign")
	os.WriteFile(foreign, []byte("retain me"), 0600)
	if err := os.Symlink(foreign, filepath.Join(dir, "history.jsonl")); err != nil {
		t.Skip("symlink unavailable")
	}
	r, err := s.Administration().Maintain(context.Background(), "purge", []string{"pick_history"}, "", true)
	if err != nil || r.Error == "" || r.Categories["pick_history"].Failed == 0 {
		t.Fatalf("partial failure discarded: %+v %v", r, err)
	}
	b, _ := os.ReadFile(foreign)
	if string(b) != "retain me" {
		t.Fatal("foreign data modified")
	}
}
