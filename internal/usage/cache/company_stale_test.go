//go:build !nousage

package cache

import (
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestCompanyCacheProducerStaleness(t *testing.T) {
	old := readCompanyPolicy
	t.Cleanup(func() { readCompanyPolicy = old })
	p := company.Defaults()
	readCompanyPolicy = func() (company.Snapshot, error) { return company.Snapshot{Managed: true, Policy: &p}, nil }
	now := time.Now().UTC()
	for _, tc := range []struct {
		name  string
		at    time.Time
		stale bool
	}{
		{"current", now, false},
		{"missing-or-invalid", time.Time{}, true},
		{"future", now.Add(time.Hour), true},
		{"producer-stale", now, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Store{Dir: t.TempDir()}
			percent := 25.0
			snap := usage.Snapshot{Provider: "claude", Source: usage.SourceWeb, FetchedAt: tc.at, Stale: tc.stale, UsageKnown: true,
				Windows: []usage.Window{{ID: "5h", Unit: usage.UnitPercent, UsedPercent: &percent, UsageKnown: true}}}
			if err := s.Write("claude", snap); err != nil {
				t.Fatal(err)
			}
			for _, ttl := range []time.Duration{time.Minute, 24 * time.Hour} {
				got, stale, err := s.Read("claude", ttl)
				if err != nil || stale != tc.stale || got.Stale != tc.stale || !got.FetchedAt.Equal(tc.at) {
					t.Errorf("ttl=%v: Read stale=%v snapshot=%+v error=%v", ttl, stale, got, err)
				}
				offline := s.OfflineRead("claude", ttl)
				if offline.Failure != nil || offline.Stale != tc.stale || !offline.FetchedAt.Equal(tc.at) {
					t.Errorf("ttl=%v: OfflineRead=%+v", ttl, offline)
				}
			}
		})
	}
}
