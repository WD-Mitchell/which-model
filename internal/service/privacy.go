//go:build !nousage

package service

import (
	"context"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"log"
	"path/filepath"
	"time"
)

func (s *Services) startPrivacyMaintenance(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	policy, err := readCompanyPolicy()
	if err != nil || !policy.Managed {
		return
	}
	s.privacyOnce.Do(func() {
		run := func() {
			layout, err := privacy.DefaultLayout()
			if err == nil {
				layout.StateDir = s.paths.StateDir
				layout.CacheDirs = append(layout.CacheDirs, filepath.Join(s.paths.CacheDir, "usage-cache"))
				if s.usageCacheDir != "" {
					layout.CacheDirs = append(layout.CacheDirs, s.usageCacheDir)
				}
				err = s.runPrivacyMaintenance(layout)
			}
			if err != nil {
				log.Print("company privacy maintenance incomplete; run privacy cleanup for category results")
			}
		}
		run()
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					run()
				}
			}
		}()
	})
}

func (s *Services) runPrivacyMaintenance(layout privacy.Layout) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	_, err = (privacy.Controller{Policy: policy}).Maintain(layout, false, nil)
	return err
}
