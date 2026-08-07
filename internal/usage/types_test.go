package usage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUnitAndSourceConstants(t *testing.T) {
	if UnitPercent != "percent" {
		t.Errorf("UnitPercent = %q, want percent", UnitPercent)
	}
	if SourceCache != "cache" {
		t.Errorf("SourceCache = %q, want cache", SourceCache)
	}
	sources := []Source{SourceOAuth, SourceAPI, SourceCLI, SourceWeb, SourceLocal, SourceCache}
	if len(sources) != 6 {
		t.Errorf("Source constants: got %d, want 6", len(sources))
	}
}

func TestKindString(t *testing.T) {
	if got := KindGateway.String(); got != "gateway" {
		t.Errorf("KindGateway.String() = %q, want gateway", got)
	}
	if got := Kind(99).String(); got != "unknown" {
		t.Errorf("Kind(99).String() = %q, want unknown", got)
	}
	if got := KindSubscription.String(); got != "subscription" {
		t.Errorf("KindSubscription.String() = %q, want subscription", got)
	}
	if got := KindAPIKeyBilling.String(); got != "api_key_billing" {
		t.Errorf("KindAPIKeyBilling.String() = %q, want api_key_billing", got)
	}
	if got := KindLocalTool.String(); got != "local_tool" {
		t.Errorf("KindLocalTool.String() = %q, want local_tool", got)
	}
}

func TestWindowZeroValue(t *testing.T) {
	var w Window
	if w.ModelScope != nil {
		t.Errorf("zero Window.ModelScope = %v, want nil", w.ModelScope)
	}
}

func TestSnapshotJSONRoundTrip(t *testing.T) {
	s := Snapshot{Provider: "x"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"provider":"x","windows":null,"fetched_at":"0001-01-01T00:00:00Z","source":"","confidence":"","usage_known":false}`
	if string(b) != want {
		t.Errorf("Snapshot JSON = %s, want %s", b, want)
	}
	for _, absent := range []string{`"account"`, `"plan"`, `"stale"`, `"error"`} {
		if strings.Contains(string(b), absent) {
			t.Errorf("omitempty field %s present in %s", absent, b)
		}
	}
	var back Snapshot
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Provider != "x" {
		t.Errorf("round-trip Provider = %q, want x", back.Provider)
	}
}

func TestFailureJSONRoundTrip(t *testing.T) {
	f := Failure{Code: "timeout", Message: "x"}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"code":"timeout","message":"x"}` {
		t.Errorf("Failure JSON = %s", b)
	}
	var back Failure
	if err := json.Unmarshal(b, &back); err != nil || back != f {
		t.Errorf("round-trip = %v, err %v, want %v", back, err, f)
	}
}
