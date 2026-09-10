// Package company owns administrator policy independently of user configuration.
// It imports no other internal package and never reads provider credentials.
package company

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

const MaxPolicyBytes = 64 * 1024

type Retention struct {
	UsageSnapshotsHours int64 `json:"usage_snapshots_hours"`
	LaunchLogsDays      int64 `json:"launch_logs_days"`
	PickHistoryDays     int64 `json:"pick_history_days"`
	AuditRecordsDays    int64 `json:"audit_records_days"`
}

type Integrations struct {
	SkillInstallation bool `json:"skill_installation"`
	HookInstallation  bool `json:"hook_installation"`
	HookUse           bool `json:"hook_use"`
}

type Installation struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Executable struct {
	ID     string   `json:"id"`
	Path   string   `json:"path"`
	SHA256 string   `json:"sha256"`
	Args   []string `json:"args"`
}

type Policy struct {
	SchemaVersion         int            `json:"schema_version"`
	Name                  string         `json:"name"`
	AllowedProviders      []string       `json:"allowed_providers"`
	CredentialSources     []string       `json:"credential_sources"`
	SecureStoreOnly       bool           `json:"secure_store_only"`
	IdentityFree          bool           `json:"identity_free"`
	Retention             Retention      `json:"retention"`
	Integrations          Integrations   `json:"integrations"`
	Executables           []Executable   `json:"executables"`
	CodexBarInstallations []Installation `json:"codexbar_installations"`
	AllowCustomShell      bool           `json:"allow_custom_shell"`
}

type Snapshot struct {
	Managed  bool    `json:"managed"`
	Required bool    `json:"required"`
	Origin   string  `json:"origin,omitempty"`
	SHA256   string  `json:"sha256,omitempty"`
	Policy   *Policy `json:"policy,omitempty"`
}

// Error contains only known diagnostic text and the fixed policy origin.
type Error struct{ Origin, Reason string }

func (e *Error) Error() string {
	if e.Origin == "" {
		return "company policy: " + e.Reason
	}
	return "company policy at " + e.Origin + ": " + e.Reason
}
func (e *Error) ExitCode() int { return 2 }

func Defaults() Policy {
	return Policy{SchemaVersion: 1, Name: "Company profile", AllowedProviders: []string{},
		CredentialSources: []string{"keychain"}, SecureStoreOnly: true, IdentityFree: true,
		Retention:   Retention{UsageSnapshotsHours: 24, LaunchLogsDays: 7, PickHistoryDays: 30, AuditRecordsDays: 30},
		Executables: []Executable{}, CodexBarInstallations: []Installation{}}
}

var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Parse retains managed defaults for omitted settings. Authority still requires
// the protected-file loader; parsing an arbitrary document does not enroll a host.
func Parse(data []byte) (Policy, error) {
	p := Defaults()
	p.SchemaVersion = 0 // version must be explicit even though other fields default
	if err := decodeStrict(data, &p); err != nil {
		return Policy{}, &Error{Reason: "invalid policy JSON, duplicate or unsupported field"}
	}
	invalid := func(field string) (Policy, error) {
		return Policy{}, &Error{Reason: "invalid administrator setting: " + field}
	}
	if p.SchemaVersion != 1 {
		return invalid("schema_version")
	}
	if p.Name == "" || len(p.Name) > 128 || strings.ContainsFunc(p.Name, unicode.IsControl) {
		return invalid("name")
	}
	seen := map[string]bool{}
	for _, id := range p.AllowedProviders {
		if !identifier.MatchString(id) || seen[id] {
			return invalid("allowed_providers")
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, source := range p.CredentialSources {
		if !slices.Contains([]string{"keychain", "environment", "provider_file", "managed_file", "cli"}, source) || seen[source] {
			return invalid("credential_sources")
		}
		seen[source] = true
	}
	if p.SecureStoreOnly && seen["managed_file"] {
		return invalid("secure_store_only")
	}
	if p.Retention.UsageSnapshotsHours < 0 || p.Retention.UsageSnapshotsHours > math.MaxInt64/int64(time.Hour) {
		return invalid("retention.usage_snapshots_hours")
	}
	for _, days := range []int64{p.Retention.LaunchLogsDays, p.Retention.PickHistoryDays, p.Retention.AuditRecordsDays} {
		if days < 0 || days > math.MaxInt64/int64(24*time.Hour) {
			return invalid("retention")
		}
	}
	validInstallation := func(path, sha string) bool {
		return filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsFunc(path, unicode.IsControl) && digestPattern.MatchString(sha)
	}
	seen = map[string]bool{}
	for _, exe := range p.Executables {
		if !identifier.MatchString(exe.ID) || seen[exe.ID] || !validInstallation(exe.Path, exe.SHA256) || len(exe.Args) > 64 {
			return invalid("executables")
		}
		seen[exe.ID] = true
		for _, arg := range exe.Args {
			if len(arg) > 4096 || strings.ContainsFunc(arg, unicode.IsControl) {
				return invalid("executables.args")
			}
		}
	}
	seen = map[string]bool{}
	for _, installation := range p.CodexBarInstallations {
		if !validInstallation(installation.Path, installation.SHA256) || seen[installation.Path] {
			return invalid("codexbar_installations")
		}
		seen[installation.Path] = true
	}
	return p, nil
}

// decodeStrict rejects duplicate object keys, unknown fields and trailing JSON.
// Diagnostics intentionally omit decoder text, which may quote private input.
func decodeStrict(data []byte, out any) error {
	if len(data) == 0 || len(data) > MaxPolicyBytes {
		return errors.New("invalid document size")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 16 {
			return errors.New("document too deep")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return errors.New("duplicate key")
				}
				keys[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("trailing data")
	}
	// All policy documents are objects; JSON null must not preserve defaults.
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return errors.New("object required")
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

// Load reads the fixed OS location. There is deliberately no path/env argument.
func Load() (Snapshot, error) {
	dir, err := policyDirectory()
	if err != nil {
		return Snapshot{}, &Error{Reason: "machine policy location is unavailable"}
	}
	if dir == "" {
		return Snapshot{}, nil
	}
	return loadAt(dir, readProtected)
}

func loadAt(dir string, read func(string) ([]byte, error)) (Snapshot, error) {
	markerPath := filepath.Join(dir, "required.json")
	policyPath := filepath.Join(dir, "policy.json")
	markerData, markerErr := read(markerPath)
	required := false
	if markerErr == nil {
		var marker struct {
			SchemaVersion int  `json:"schema_version"`
			Required      bool `json:"required"`
		}
		if decodeStrict(markerData, &marker) != nil || marker.SchemaVersion != 1 || !marker.Required {
			return Snapshot{}, &Error{markerPath, "invalid enrollment marker; contact the administrator"}
		}
		required = true
	} else if !errors.Is(markerErr, os.ErrNotExist) {
		return Snapshot{}, &Error{markerPath, "enrollment is unreadable or insufficiently protected; contact the administrator"}
	}
	data, err := read(policyPath)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, nil
		}
		return Snapshot{}, &Error{policyPath, "required policy is absent, unreadable or insufficiently protected; contact the administrator"}
	}
	p, err := Parse(data)
	if err != nil {
		return Snapshot{}, &Error{policyPath, "invalid policy; contact the administrator"}
	}
	sum := sha256.Sum256(data)
	return Snapshot{Managed: true, Required: required, Origin: policyPath, SHA256: hex.EncodeToString(sum[:]), Policy: &p}, nil
}

func (s Snapshot) denied(setting string) error {
	return &Error{s.Origin, "operation is prohibited by " + setting}
}
func (s Snapshot) RequireProvider(provider string) error {
	if !s.Managed {
		return nil
	}
	if s.Policy == nil || !slices.Contains(s.Policy.AllowedProviders, provider) {
		return s.denied("allowed_providers")
	}
	return nil
}
func (s Snapshot) RequireSource(source string) error {
	if !s.Managed {
		return nil
	}
	if source == "cli" {
		return s.denied("legacy credential command has no verified executable approval")
	}
	if s.Policy == nil || !slices.Contains(s.Policy.CredentialSources, source) || (source == "managed_file" && s.Policy.SecureStoreOnly) {
		return s.denied("credential_sources / secure_store_only")
	}
	return nil
}
func (s Snapshot) RequireCapability(capability string) error {
	if !s.Managed {
		return nil
	}
	if s.Policy == nil {
		return s.denied("required policy")
	}
	allowed := false
	switch capability {
	case "skill_installation":
		allowed = s.Policy.Integrations.SkillInstallation
	case "hook_installation":
		allowed = s.Policy.Integrations.HookInstallation
	case "hook_use":
		allowed = s.Policy.Integrations.HookUse
	// Legacy launch/delegation cannot treat metadata as an executable approval.
	// #285/#287 replace these guards with their verified operation implementations.
	case "harness_launch", "codexbar":
		allowed = false
	}
	if !allowed {
		return s.denied("managed capability permissions")
	}
	return nil
}
