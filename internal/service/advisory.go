package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"time"

	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"github.com/WD-Mitchell/which-model/internal/routing"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/toggle"
	"github.com/oklog/ulid/v2"
)

// observeUsage retains only the latest typed observation per requested provider.
// It never preserves account/plan or provider error text and never writes a file.
func (s *Services) observeUsage(providers []string, snapshots []usage.Snapshot, maxAge time.Duration, fetchErr error) {
	policy, err := readCompanyPolicy()
	if err != nil || !policy.Managed {
		return
	}
	observed := map[string]usage.Snapshot{}
	for _, snap := range snapshots {
		snap.Account = ""
		snap.Plan = ""
		snap.Windows = append([]usage.Window(nil), snap.Windows...)
		if snap.Failure != nil {
			snap.Failure = &usage.Failure{Code: snap.Failure.Code}
		}
		observed[snap.Provider] = snap
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.usageObservations == nil {
		s.usageObservations = map[string]usage.Snapshot{}
	}
	if s.usageObservationMaxAge == nil {
		s.usageObservationMaxAge = map[string]time.Duration{}
	}
	for _, provider := range providers {
		snap, ok := observed[provider]
		if !ok {
			delete(s.usageObservations, provider)
			delete(s.usageObservationMaxAge, provider)
			if fetchErr != nil {
				s.usageObservations[provider] = usage.Snapshot{Provider: provider, Failure: &usage.Failure{Code: "provider_status"}}
			}
			continue
		}
		s.usageObservations[provider] = snap
		s.usageObservationMaxAge[provider] = maxAge
	}
}
func (s *Services) routeEvidenceLocked(route routing.Route, now time.Time) advisory.Report {
	enabled, _ := toggle.ResolveUsageEnabled(false, s.cfg)
	if !s.cfg.Providers[route.Provider].Enabled {
		enabled = false
	}
	var snapshot *usage.Snapshot
	if value, ok := s.usageObservations[route.Provider]; ok {
		snapshot = &value
	}
	maxAge := s.usageObservationMaxAge[route.Provider]
	if maxAge <= 0 {
		maxAge = 15 * time.Minute
	}
	return advisory.Evaluate(enabled, snapshot, route.WindowIDs, maxAge, now)
}
func (s *Services) launchEvidence(provider, model, reasoning string) advisory.Report {
	s.mu.RLock()
	defer s.mu.RUnlock()
	route := routing.Route{Provider: provider, ModelID: model, Reasoning: reasoning}
	for _, candidate := range s.routes.Routes {
		if candidate.Provider == provider && candidate.ModelID == model && candidate.Reasoning == reasoning {
			route = candidate
			break
		}
	}
	return s.routeEvidenceLocked(route, time.Now())
}

type launchAuditRecord struct {
	LaunchID   string `json:"launch_id"`
	Candidate  string `json:"candidate"`
	Profile    string `json:"profile"`
	Phase      string `json:"phase"`
	QuotaState string `json:"quota_state"`
}

var writeLaunchAudit = func(state string, policy company.Snapshot, record launchAuditRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	err = (privacy.Controller{Policy: policy}).Append(filepath.Join(state, "audit", "evidence.jsonl"), privacy.Audit, data)
	if err == nil && policy.Policy.Retention.AuditRecordsDays == 0 {
		return errors.New("audit persistence disabled")
	}
	return err
}

func newLaunchAudit(provider, model, profile string, report advisory.Report) launchAuditRecord {
	return launchAuditRecord{LaunchID: ulid.Make().String(), Candidate: provider + ":" + model, Profile: profile, QuotaState: report.State}
}
func (h *HarnessService) auditLaunch(policy company.Snapshot, record launchAuditRecord, phase string) error {
	record.Phase = phase
	return writeLaunchAudit(h.s.paths.StateDir, policy, record)
}
func launchAdvisories(report advisory.Report) []string {
	if report.State == advisory.Current {
		return nil
	}
	return []string{report.Message}
}
func (h *HarnessService) recordLaunchOutcome(ctx context.Context, slug, provider, model, profile, routeKey, outcome string, messages []string) []string {
	if err := h.recordCompanyLaunch(slug, provider, model, profile, outcome); err != nil {
		messages = append(messages, "Launch log was not recorded.")
	}
	if outcome != "failed" {
		if err := h.recordPick(ctx, profile, routeKey); err != nil {
			messages = append(messages, "Pick history was not recorded.")
		}
	}
	return messages
}
