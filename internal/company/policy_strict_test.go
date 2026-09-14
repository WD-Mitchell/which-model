package company

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyRejectsCaseAliasedFields(t *testing.T) {
	for _, input := range []string{
		`{"schema_version":1,"integrations":{"hook_use":false,"HOOK_USE":true}}`,
		`{"schema_version":1,"allowed_providers":[],"ALLOWED_PROVIDERS":["copilot"]}`,
		`{"schema_version":1,"secure_store_only":true,"SECURE_STORE_ONLY":false,"credential_sources":["managed_file"]}`,
		`{"schema_version":1,"\u017fecure_store_only":false,"credential_sources":["managed_file"]}`,
		`{"schema_version":2,"SCHEMA_VERSION":1}`,
		`{"SCHEMA_VERSION":1}`,
		`{"schema_version":1,"retention":{"PICK_HISTORY_DAYS":0}}`,
		`{"schema_version":1,"NAME":"REJECTED_POLICY_CANARY"}`,
	} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Errorf("accepted noncanonical policy: %s", input)
		} else if strings.Contains(err.Error(), "CANARY") {
			t.Fatal("diagnostic exposed policy contents")
		}
	}
	// JSON escapes may spell a canonical property; they cannot create another
	// spelling of the same field that bypasses duplicate detection.
	if _, err := Parse([]byte(`{"schema_version":1,"integrations":{"hook\u005fuse":true}}`)); err != nil {
		t.Fatal("canonical escaped field rejected:", err)
	}
	if _, err := Parse([]byte(`{"schema_version":1,"integrations":{"hook_use":false,"hook\u005fuse":true}}`)); err == nil {
		t.Fatal("escaped duplicate accepted")
	}
}

func TestEnrollmentRejectsCaseAliasedFields(t *testing.T) {
	for _, marker := range []string{
		`{"schema_version":1,"required":false,"REQUIRED":true}`,
		`{"SCHEMA_VERSION":1,"required":true}`,
	} {
		_, err := loadAt(t.TempDir(), func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "required.json":
				return []byte(marker), nil
			case "policy.json":
				return []byte(`{"schema_version":1}`), nil
			default:
				return nil, os.ErrNotExist
			}
		})
		if err == nil {
			t.Errorf("accepted noncanonical enrollment: %s", marker)
		}
	}
}
