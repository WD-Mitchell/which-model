//go:build !nousage

package fetch

import (
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"strings"
	"testing"
)

func TestCompanyDiagnosticsExcludeProviderPayload(t *testing.T) {
	snapshots := []usage.Snapshot{{Failure: &usage.Failure{Code: "unauthorized", Message: "RAW_CREDENTIAL_CANARY"}}, {Failure: &usage.Failure{Code: "SECRET_CODE_CANARY", Message: "UNRELATED_PAYLOAD_CANARY"}}, {Failure: &usage.Failure{Code: "keychain_unavailable", Message: (&securestore.Error{Kind: securestore.Locked}).Error()}}}
	minimizeCompanyFailures(snapshots)
	for _, s := range snapshots {
		if strings.Contains(s.Failure.Code, "CANARY") || strings.Contains(s.Failure.Message, "CANARY") {
			t.Fatal("raw provider failure escaped")
		}
	}
	if snapshots[0].Failure.Code != "unauthorized" || snapshots[1].Failure.Code != "provider_status" {
		t.Fatal("failure code mapping changed")
	}
	if snapshots[2].Failure.Message != (&securestore.Error{Kind: securestore.Locked}).Error() {
		t.Fatal("native lock remediation lost")
	}
}
