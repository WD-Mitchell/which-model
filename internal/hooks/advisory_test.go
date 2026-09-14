package hooks

import (
	"github.com/WD-Mitchell/which-model/internal/company"
	"strings"
	"testing"
	"time"
)

func TestCompanyDispatchEvidenceNeverBlocks(t *testing.T) {
	p := company.Defaults()
	old := readPrivacyPolicy
	t.Cleanup(func() { readPrivacyPolicy = old })
	readPrivacyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	for _, test := range []struct {
		name       string
		code       int
		data, want string
	}{
		{"usage-refresh", 1, `RAW_PROVIDER_CANARY`, "unavailable"},
		{"quota-guard", 0, `{"snapshots":[{"provider":"codex","fetched_at":"` + time.Now().UTC().Format(time.RFC3339) + `","usage_known":true,"windows":[{"id":"session","usage_known":true,"used_percent":99}]}]}`, "advisory"},
		{"quota-guard", 5, `{"snapshots":[{"provider":"codex","error":{"code":"unauthorized","message":"SECRET_CANARY"}}]}`, "authentication_error"},
		{"spawn-gate", 4, `{"excluded_candidates":[{"reason_code":"band_gated","reason":"SECRET_CANARY"}]}`, "Recommendation evidence is unavailable"},
		{"spawn-gate", 0, `{"usage_enabled":false,"candidates":[{"candidate_id":"codex:model","route":{"provider":"codex","model_id":"model","reasoning":"high"},"final_score":80,"raw_output":"SECRET_CANARY"}]}`, "Score-only recommendation"},
		{"model-audit", 1, `RAW_PROVIDER_CANARY`, "audit_recorded"},
	} {
		t.Run(test.name+test.want, func(t *testing.T) {
			h, _ := Get(test.name)
			out, err := dispatch(h, test.code, []byte(test.data), Options{})
			if err != nil || !strings.Contains(string(out), `"decision":"approve"`) || !strings.Contains(string(out), test.want) || strings.Contains(string(out), "CANARY") {
				t.Fatalf("advisory outcome: %s %v", out, err)
			}
		})
	}
}
