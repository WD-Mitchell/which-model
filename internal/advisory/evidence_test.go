package advisory

import (
	"github.com/WD-Mitchell/which-model/internal/usage"
	"strings"
	"testing"
	"time"
)

func TestCompanyAdvisoryMatrix(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	percent := 25.0
	fresh := func() *usage.Snapshot {
		return &usage.Snapshot{FetchedAt: now.Add(-time.Minute), UsageKnown: true, Windows: []usage.Window{{ID: "session", UsageKnown: true, UsedPercent: &percent}}}
	}
	for _, test := range []struct {
		name    string
		enabled bool
		adjust  func(*usage.Snapshot) *usage.Snapshot
		ids     []string
		state   string
	}{
		{"current", true, func(s *usage.Snapshot) *usage.Snapshot { return s }, []string{"session"}, Current},
		{"unknown snapshot", true, func(s *usage.Snapshot) *usage.Snapshot { s.UsageKnown = false; return s }, []string{"session"}, Unknown},
		{"missing", true, func(*usage.Snapshot) *usage.Snapshot { return nil }, []string{"session"}, Missing},
		{"unknown windows", true, func(s *usage.Snapshot) *usage.Snapshot { s.Windows = nil; return s }, []string{"session"}, Unknown},
		{"partial", true, func(s *usage.Snapshot) *usage.Snapshot { return s }, []string{"session", "weekly"}, Partial},
		{"stale flag", true, func(s *usage.Snapshot) *usage.Snapshot { s.Stale = true; return s }, []string{"session"}, Stale},
		{"age", true, func(s *usage.Snapshot) *usage.Snapshot { s.FetchedAt = now.Add(-16 * time.Minute); return s }, []string{"session"}, Stale},
		{"future", true, func(s *usage.Snapshot) *usage.Snapshot { s.FetchedAt = now.Add(time.Minute); return s }, []string{"session"}, Stale},
		{"authentication", true, func(s *usage.Snapshot) *usage.Snapshot {
			s.Failure = &usage.Failure{Code: "unauthorized", Message: "SECRET_CANARY"}
			return s
		}, []string{"session"}, AuthenticationError},
		{"secure store", true, func(s *usage.Snapshot) *usage.Snapshot {
			s.Failure = &usage.Failure{Code: "keychain_unavailable", Message: "SECRET_CANARY"}
			return s
		}, []string{"session"}, AuthenticationError},
		{"provider", true, func(s *usage.Snapshot) *usage.Snapshot {
			s.Failure = &usage.Failure{Code: "network", Message: "SECRET_CANARY"}
			return s
		}, []string{"session"}, ProviderError},
		{"disabled", false, func(s *usage.Snapshot) *usage.Snapshot { return s }, []string{"session"}, Disabled},
		{"synthetic", true, func(s *usage.Snapshot) *usage.Snapshot { s.Windows[0].Synthetic = true; return s }, []string{"session"}, Unknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := Evaluate(test.enabled, test.adjust(fresh()), test.ids, 15*time.Minute, now)
			if report.State != test.state || report.Message == "" || strings.Contains(report.Message, "CANARY") {
				t.Fatalf("report=%+v want %s", report, test.state)
			}
		})
	}
	// A healthy route never supplies a result for a missing one.
	healthy := Evaluate(true, fresh(), []string{"session"}, 15*time.Minute, now)
	missing := Evaluate(true, nil, []string{"session"}, 15*time.Minute, now)
	if healthy.State != Current || missing.State != Missing {
		t.Fatal("mixed route state was shared")
	}
}
