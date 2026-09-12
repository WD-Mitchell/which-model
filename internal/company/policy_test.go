package company

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedDefaultsAndAuthority(t *testing.T) {
	p, err := Parse([]byte(`{"schema_version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !p.SecureStoreOnly || !p.IdentityFree || p.Retention.UsageSnapshotsHours != 24 || p.Retention.LaunchLogsDays != 7 || p.Retention.PickHistoryDays != 30 || p.Retention.AuditRecordsDays != 30 {
		t.Fatal("managed defaults differ from approved decisions")
	}
	s := Snapshot{Managed: true, Policy: &p}
	if s.RequireSource("keychain") != nil {
		t.Fatal("keychain should be allowed")
	}
	for _, source := range []string{"managed_file", "provider_file", "environment", "cli"} {
		if s.RequireSource(source) == nil {
			t.Errorf("source %s allowed", source)
		}
	}
	if s.RequireProvider("codex") == nil || s.RequireCapability("skill_installation") == nil {
		t.Fatal("missing default deny")
	}
	personal := Snapshot{}
	if personal.RequireSource("managed_file") != nil || personal.RequireProvider("codex") != nil || personal.RequireCapability("harness_launch") != nil {
		t.Fatal("personal behavior restricted")
	}
}

func TestPolicyRejectsAmbiguousOrUnsupportedAuthority(t *testing.T) {
	for _, input := range []string{
		`{}`, `{"schema_version":2}`, `{"schema_version":1,"unknown":"CANARY"}`,
		`{"schema_version":1,"schema_version":1}`, `{"schema_version":1,"integrations":{"hook_use":true,"hook_use":false}}`,
		`{"schema_version":1,"credential_sources":["CANARY"]}`,
		`{"schema_version":1,"credential_sources":["managed_file"]}`,
		`{"schema_version":1,"retention":{"launch_logs_days":-1}}`,
		`{"schema_version":1,"retention":{"usage_snapshots_hours":9223372036854775807}}`,
		`{"schema_version":1,"allowed_providers":["codex","codex"]}`,
		`{"schema_version":1} {"schema_version":1}`, strings.Repeat(" ", MaxPolicyBytes+1),
	} {
		_, err := Parse([]byte(input))
		if err == nil {
			t.Errorf("accepted invalid policy %s", input[:min(len(input), 80)])
		} else if strings.Contains(err.Error(), "CANARY") {
			t.Fatal("error echoed policy data")
		}
	}
}

func TestExplicitAdministratorChoices(t *testing.T) {
	p, err := Parse([]byte(`{"schema_version":1,"allowed_providers":["codex"],"credential_sources":["keychain","provider_file"],"integrations":{"skill_installation":true},"identity_free":false,"retention":{"usage_snapshots_hours":0}}`))
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{Managed: true, Policy: &p}
	if s.RequireProvider("codex") != nil || s.RequireSource("provider_file") != nil || s.RequireCapability("skill_installation") != nil {
		t.Fatal("administrator allowance lost")
	}
	if s.RequireProvider("claude") == nil || s.RequireSource("managed_file") == nil || s.RequireCapability("hook_installation") == nil {
		t.Fatal("allowance broadened")
	}
	if p.IdentityFree || p.Retention.UsageSnapshotsHours != 0 || p.Retention.AuditRecordsDays != 30 {
		t.Fatal("explicit fields or defaults lost")
	}
}

func TestEnrollmentRequiresTrustedPolicy(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{}
	read := func(path string) ([]byte, error) {
		b, ok := files[filepath.Base(path)]
		if !ok {
			return nil, os.ErrNotExist
		}
		return b, nil
	}
	s, err := loadAt(dir, read)
	if err != nil || s.Managed {
		t.Fatal("no enrollment should be personal", err)
	}
	files["required.json"] = []byte(`{"schema_version":1,"required":true}`)
	if _, err := loadAt(dir, read); err == nil {
		t.Fatal("missing required policy accepted")
	}
	files["policy.json"] = []byte(`{"schema_version":1}`)
	s, err = loadAt(dir, read)
	if err != nil || !s.Managed || !s.Required || s.Origin != filepath.Join(dir, "policy.json") || len(s.SHA256) != 64 {
		t.Fatal("trusted enrollment not loaded", err)
	}
	delete(files, "required.json")
	s, err = loadAt(dir, read)
	if err != nil || !s.Managed || s.Required {
		t.Fatal("standalone protected policy not active", err)
	}
	for _, bad := range []string{`{"schema_version":1,"required":false}`, `{"schema_version":1,"required":true,"unknown":"CANARY"}`} {
		files["required.json"] = []byte(bad)
		if _, err := loadAt(dir, read); err == nil || strings.Contains(err.Error(), "CANARY") {
			t.Fatal("invalid marker not safely refused", err)
		}
	}
	_, err = loadAt(dir, func(string) ([]byte, error) { return nil, errors.New("CANARY unsafe permissions") })
	if err == nil || strings.Contains(err.Error(), "CANARY") {
		t.Fatal("protection failure must be sanitized and fatal")
	}
}
