package service

import (
	"context"
	"log"
	"time"
)

func (s *Services) dataRefreshInterval() time.Duration {
	s.mu.RLock()
	gui, err := s.cfg.LoadGUI()
	s.mu.RUnlock()
	if err != nil {
		return 6 * time.Hour
	}
	if gui.BenchmarkCheckFrequency == "weekly" {
		return 7 * 24 * time.Hour
	}
	interval, err := time.ParseDuration(gui.BenchmarkCheckFrequency)
	if err != nil || interval < 15*time.Minute {
		return 6 * time.Hour
	}
	return interval
}

// StartDataRefresher refreshes the selected source at startup and at the user's
// configured interval. A minute tick observes interval changes without restart;
// refreshes are serialized with manual operations by RefreshRoutes.
func (s *Services) StartDataRefresher(ctx context.Context) {
	s.dataRefresherOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			s.runDataRefresher(ctx, ticker.C, time.Now(), func(ctx context.Context) {
				runCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
				defer cancel()
				if err := s.Providers().RefreshRoutes(runCtx); err != nil && ctx.Err() == nil {
					log.Printf("data refresh: %v", err)
				}
			})
		}()
	})
}

func (s *Services) runDataRefresher(ctx context.Context, ticks <-chan time.Time, now time.Time, refresh func(context.Context)) {
	if ctx.Err() != nil {
		return
	}
	refresh(ctx)
	last := now
	for {
		select {
		case <-ctx.Done():
			return
		case now, ok := <-ticks:
			if !ok {
				return
			}
			if now.Sub(last) >= s.dataRefreshInterval() {
				if ctx.Err() != nil {
					return
				}
				refresh(ctx)
				last = now
			}
		}
	}
}
