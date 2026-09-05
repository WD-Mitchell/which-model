package service

import (
	"context"
	"testing"
	"time"
)

func TestDataRefresherUsesConfiguredIntervalAndStops(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`[gui]
benchmark_check_frequency = "15m"
`))
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	ticks := make(chan time.Time, 3)
	ticks <- now.Add(14 * time.Minute)
	ticks <- now.Add(15 * time.Minute)
	ticks <- now.Add(29 * time.Minute)
	close(ticks)
	calls := 0
	svc.runDataRefresher(context.Background(), ticks, now, func(context.Context) { calls++ })
	if calls != 2 {
		t.Fatalf("refresh calls=%d, want startup plus 15-minute refresh", calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.runDataRefresher(ctx, ticks, now, func(context.Context) { t.Fatal("refresh after cancellation") })
	gui, err := svc.cfg.LoadGUI()
	if err != nil {
		t.Fatal(err)
	}
	gui.BenchmarkCheckFrequency = "weekly"
	if err := svc.cfg.SetGUI(gui); err != nil {
		t.Fatal(err)
	}
	if got := svc.dataRefreshInterval(); got != 7*24*time.Hour {
		t.Fatalf("changed interval=%v", got)
	}
}
