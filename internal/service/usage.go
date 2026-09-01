package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/cache"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
)

// UsageMode is the current usage toggle and backend.
type UsageMode struct {
	Mode    string `json:"mode"`
	Backend string `json:"backend"`
}
type UsageService struct{ s *Services }

func (s *Services) Usage() *UsageService { return &UsageService{s: s} }

// Direct Services wrappers retained for the host-facing API.
func (s *Services) Snapshots(ctx context.Context, force bool) ([]UsageDTO, error) {
	return s.Usage().Snapshots(ctx, force)
}
func (s *Services) History(ctx context.Context, provider, window string) ([]UsageWindow, error) {
	return s.Usage().History(ctx, provider, window)
}
func (s *Services) Mode(ctx context.Context) (UsageMode, error) { return s.Usage().Mode(ctx) }
func (s *Services) SetMode(ctx context.Context, mode string) error {
	return s.Usage().SetMode(ctx, mode)
}
func (s *Services) Backend(ctx context.Context) (string, error) { return s.Usage().Backend(ctx) }
func (s *Services) SetBackend(ctx context.Context, backend string) error {
	return s.Usage().SetBackend(ctx, backend)
}

type usageLockContextKey struct{}

func (u *UsageService) Snapshots(ctx context.Context, force bool) ([]UsageDTO, error) {
	s := u.s
	s.mu.RLock()
	cfg := s.cfg
	providers := make([]string, 0)
	enabled := map[string]bool{}
	for id, p := range cfg.Providers {
		if p.Enabled {
			providers = append(providers, id)
			enabled[id] = true
		}
	}
	backend := cfg.Usage.Backend
	auth, authErr := cfg.LoadAuth()
	stateDir := s.paths.StateDir
	s.mu.RUnlock()
	if authErr != nil {
		return nil, authErr
	}
	ok, reason := toggle.ResolveUsageEnabled(false, cfg)
	if !ok {
		return nil, fmt.Errorf("%w: usage disabled: %s", errUsageUnavailable, reason)
	}
	sort.Slice(providers, func(i, j int) bool {
		s.mu.RLock()
		pi := cfg.Providers[providers[i]].Priority
		pj := cfg.Providers[providers[j]].Priority
		s.mu.RUnlock()
		if pi != pj {
			return pi < pj
		}
		return providers[i] < providers[j]
	})
	locked := ctx.Value(usageLockContextKey{}) != nil
	if !locked {
		s.usageFetchMu.Lock()
		defer s.usageFetchMu.Unlock()
	}
	dir := s.usageCacheDir
	if dir == "" {
		var err error
		dir, err = cache.CacheDir()
		if err != nil {
			return nil, err
		}
	}
	maxAge := 15 * time.Minute
	if force {
		maxAge = time.Minute
	}
	snaps, warns, err := fetch.FetchAll(ctx, providers, fetch.Options{Backend: backend, Enabled: enabled, MaxAge: maxAge, Timeout: 10 * time.Second, CacheDir: dir, StateDir: stateDir, DisableManagedKeychain: !auth.UseKeychain, ShowIdentity: true})
	for _, w := range warns {
		log.Printf("usage: %s", w.Message)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUsageUnavailable, err)
	}
	byID := make(map[string]usage.Snapshot, len(snaps))
	for _, v := range snaps {
		byID[v.Provider] = v
	}
	out := make([]UsageDTO, 0, len(snaps))
	now := time.Now()
	for _, id := range providers {
		if v, ok := byID[id]; ok {
			d := snapshotToDTO(v, nil, now)
			if spec, e := usage.Get(id); e == nil {
				d = snapshotToDTO(v, spec.Windows, now)
			}
			out = append(out, d)
		}
	}
	for _, v := range snaps {
		if _, ok := enabled[v.Provider]; !ok {
			d := snapshotToDTO(v, nil, now)
			out = append(out, d)
		}
	}
	if force {
		s.emit(EventUsageUpdated, struct{}{})
	}
	return out, nil
}

func (u *UsageService) History(ctx context.Context, provider, window string) ([]UsageWindow, error) {
	ds, err := u.Snapshots(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if d.Provider == provider {
			if window == "" {
				return append([]UsageWindow(nil), d.Windows...), nil
			}
			for _, w := range d.Windows {
				if w.ID == window {
					return []UsageWindow{w}, nil
				}
			}
			return []UsageWindow{}, nil
		}
	}
	return []UsageWindow{}, nil
}

func (u *UsageService) Mode(ctx context.Context) (UsageMode, error) {
	u.s.mu.RLock()
	defer u.s.mu.RUnlock()
	m := "auto"
	switch u.s.cfg.Usage.Enabled {
	case config.UsageTrue:
		m = "on"
	case config.UsageFalse:
		m = "off"
	}
	b := string(u.s.cfg.Usage.Backend)
	if b == "" {
		b = "off"
	}
	return UsageMode{Mode: m, Backend: b}, nil
}
func (u *UsageService) Backend(ctx context.Context) (string, error) {
	u.s.mu.RLock()
	defer u.s.mu.RUnlock()
	b := string(u.s.cfg.Usage.Backend)
	if b == "" {
		b = "off"
	}
	return b, nil
}
func (u *UsageService) SetMode(ctx context.Context, mode string) error {
	var v config.UsageEnabled
	switch mode {
	case "auto":
		v = config.UsageAuto
	case "on":
		v = config.UsageTrue
	case "off":
		v = config.UsageFalse
	default:
		return fmt.Errorf("%w: usage: mode %q must be one of \"auto\", \"on\", \"off\"", errValidation, mode)
	}
	s := u.s
	s.mu.Lock()
	next, cleanup, err := cloneConfig(s.cfg)
	if err == nil {
		next.Usage.Enabled = v
		var data []byte
		data, err = next.MarshalTOML()
		if err == nil {
			err = config.AtomicWriteFile(s.paths.UserConfigFile, data)
		}
	}
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		s.cfg = next
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.emit(EventConfigChanged, map[string]string{"section": "usage"})
	return nil
}
func (u *UsageService) SetBackend(ctx context.Context, backend string) error {
	var v config.UsageBackend
	switch backend {
	case "off":
		v = config.UsageBackendOff
	case "native":
		v = config.UsageBackendNative
	case "codexbar":
		v = config.UsageBackendCodexBar
	default:
		return fmt.Errorf("%w: usage: backend %q must be one of \"off\", \"native\", \"codexbar\"", errValidation, backend)
	}
	s := u.s
	s.mu.Lock()
	next, cleanup, err := cloneConfig(s.cfg)
	if err == nil {
		next.Usage.Backend = v
		var data []byte
		data, err = next.MarshalTOML()
		if err == nil {
			err = config.AtomicWriteFile(s.paths.UserConfigFile, data)
		}
	}
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		s.cfg = next
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.emit(EventConfigChanged, map[string]string{"section": "usage"})
	return nil
}

// StartRefresher runs an immediate usage refresh followed by periodic
// refreshes. The host may combine this loop with catalog refresh handling.
func (s *Services) StartRefresher(ctx context.Context, interval time.Duration) {
	s.refresherOnce.Do(func() {
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go func() {
			run := func() {
				if _, err := s.Usage().Snapshots(ctx, false); err == nil {
					s.emit(EventUsageUpdated, struct{}{})
				}
			}
			run()
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if !s.usageFetchMu.TryLock() {
						continue
					}
					runCtx := context.WithValue(ctx, usageLockContextKey{}, true)
					if _, err := s.Usage().Snapshots(runCtx, false); err == nil {
						s.emit(EventUsageUpdated, struct{}{})
					}
					s.usageFetchMu.Unlock()
				}
			}
		}()
	})
}

func snapshotToDTO(snap usage.Snapshot, specs []usage.WindowSpec, now time.Time) UsageDTO {
	d := UsageDTO{Provider: snap.Provider, Plan: snap.Plan, Confidence: snap.Confidence, Stale: snap.Stale, Auth: sourceAuth(snap.Source)}
	if snap.Failure != nil {
		d.Failure = snap.Failure.Message
		if d.Failure == "" {
			d.Failure = snap.Failure.Code
		}
	}
	order := make([]usage.Window, 0, len(snap.Windows))
	seen := map[string]bool{}
	for _, sp := range specs {
		for _, w := range snap.Windows {
			if w.ID == sp.ID && !seen[w.ID] {
				order = append(order, w)
				seen[w.ID] = true
				break
			}
		}
	}
	for _, w := range snap.Windows {
		if !seen[w.ID] {
			order = append(order, w)
			seen[w.ID] = true
		}
	}
	var soon *time.Time
	var soonID string
	for _, w := range order {
		if w.Synthetic {
			continue
		}
		uw := UsageWindow{ID: w.ID, Label: w.Label, Unlimited: w.Unlimited}
		if uw.Label == "" {
			uw.Label = w.ID
		}
		if w.ResetHint != "" {
			uw.ResetHint = w.ResetHint
		} else if w.ResetsAt != nil {
			uw.ResetHint = resetHint(*w.ResetsAt, now)
		}
		if w.Unlimited {
		} else if !w.UsageKnown {
		} else if w.UsedPercent != nil {
			v := int(math.Round(*w.UsedPercent))
			if v < 0 {
				v = 0
			}
			uw.UsedPercent = &v
		} else if w.Used != nil && w.Limit != nil && *w.Limit > 0 {
			v := int(math.Round(*w.Used / *w.Limit * 100))
			if v < 0 {
				v = 0
			}
			uw.UsedPercent = &v
		} else if w.Remaining != nil && w.Limit != nil && *w.Limit > 0 {
			v := int(math.Round((*w.Limit - *w.Remaining) / *w.Limit * 100))
			if v < 0 {
				v = 0
			}
			uw.UsedPercent = &v
		}
		d.Windows = append(d.Windows, uw)
		if w.ResetsAt != nil && (soon == nil || w.ResetsAt.Before(*soon)) {
			x := *w.ResetsAt
			soon = &x
			soonID = w.ID
		}
		if d.Credits == "" && w.UsageKnown && (w.Unit == usage.UnitCredits || w.Unit == usage.UnitUSD) {
			if w.Remaining != nil {
				if w.Unit == usage.UnitCredits {
					d.Credits = fmt.Sprintf("%d credits left", int(math.Round(*w.Remaining)))
				} else {
					d.Credits = fmt.Sprintf("$%.2f left", *w.Remaining)
				}
			} else if w.Used != nil && w.Limit != nil {
				if w.Unit == usage.UnitCredits {
					d.Credits = fmt.Sprintf("%d of %d credits", int(math.Round(*w.Used)), int(math.Round(*w.Limit)))
				} else {
					d.Credits = fmt.Sprintf("$%.2f of $%.2f", *w.Used, *w.Limit)
				}
			}
		}
	}
	if soon != nil {
		d.Resets = fmt.Sprintf("%s %s", soonID, resetHint(*soon, now))
	} else {
		for _, w := range d.Windows {
			if w.ResetHint != "" {
				d.Resets = w.ID + " " + w.ResetHint
				break
			}
		}
	}
	return d
}
func sourceAuth(s usage.Source) string {
	switch s {
	case usage.SourceOAuth:
		return "oauth"
	case usage.SourceAPI:
		return "api key"
	case usage.SourceCLI:
		return "cli"
	case usage.SourceWeb:
		return "browser"
	case usage.SourceLocal:
		return "local"
	case usage.SourceCache:
		return "cached"
	}
	return ""
}
func resetHint(t, now time.Time) string {
	d := t.Sub(now)
	if d <= time.Minute {
		return "resets soon"
	}
	h := int(d / time.Hour)
	m := int((d - time.Duration(h)*time.Hour) / time.Minute)
	if h > 0 {
		return fmt.Sprintf("resets in %dh %dm", h, m)
	}
	return fmt.Sprintf("resets in %dm", m)
}
