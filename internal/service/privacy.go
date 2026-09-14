//go:build !nousage

package service

import (
	"context"
	"fmt"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"log"

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
			layout, err := s.privacyLayout("")
			if err == nil {
				err = s.runPrivacyMaintenance(layout)
			} else {
				s.privacyMu.Lock()
				s.lastMaintenance = &MaintenanceResult{Managed: true, Operation: "cleanup", CompletedAt: time.Now().UTC().Format(time.RFC3339), Categories: map[string]privacy.Report{}, Error: err.Error()}
				s.privacyMu.Unlock()
			}
			if err != nil {
				log.Print("company privacy maintenance incomplete; see Security & privacy for category results")
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
	result := s.maintainWithReport(privacy.Controller{Policy: policy}, layout, "cleanup", nil)
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}
