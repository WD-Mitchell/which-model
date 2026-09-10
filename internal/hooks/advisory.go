package hooks

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/advisory"
	"github.com/WD-Mitchell/which-model/internal/approvedexec"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func companyDispatch(policy company.Snapshot, h Hook, code int, out []byte, opts Options) ([]byte, error) {
	if len(out) > 4<<20 {
		out = nil
		code = 1
	}
	approve := func(reason string, fields map[string]any) ([]byte, error) {
		return MarshalEnvelope(Envelope{Decision: "approve", Reason: reason, HookSpecificOutput: fields}), nil
	}
	switch h.ID {
	case "usage-refresh", "quota-guard":
		var doc struct {
			Snapshots []usage.Snapshot `json:"snapshots"`
		}
		parsed := json.Unmarshal(out, &doc) == nil && doc.Snapshots != nil
		reports := map[string]advisory.Report{}
		if parsed {
			for _, snap := range doc.Snapshots {
				if !approvedexec.ValidValue(snap.Provider) || len(reports) >= 64 {
					continue
				}
				ids := make([]string, 0, len(snap.Windows))
				for _, window := range snap.Windows {
					ids = append(ids, window.ID)
				}
				reports[snap.Provider] = advisory.Evaluate(true, &snap, ids, 0, time.Now())
			}
		}
		reason := "Quota evidence is advisory; native permissions remain authoritative."
		if !parsed || code != 0 {
			reason = "Quota or refresh evidence is unavailable; dispatch remains advisory."
		} else if len(reports) == 0 {
			reason = "No quota signal was returned; full allowance was not verified. Dispatch remains advisory."
		}
		return approve(reason, map[string]any{"quota_evidence": reports, "evidence_available": parsed && len(reports) > 0})
	case "spawn-gate":
		var doc struct {
			UsageEnabled bool `json:"usage_enabled"`
			Candidates   []struct {
				CandidateID string `json:"candidate_id"`
				Route       struct {
					Provider  string `json:"provider"`
					ModelID   string `json:"model_id"`
					Reasoning string `json:"reasoning"`
				} `json:"route"`
				ModelScore float64  `json:"model_score"`
				FinalScore float64  `json:"final_score"`
				Warnings   []string `json:"warnings"`
			} `json:"candidates"`
		}
		unavailable := func() ([]byte, error) {
			return approve("Recommendation evidence is unavailable; dispatch remains advisory.", map[string]any{"recommendation_available": false})
		}
		if code != 0 || json.Unmarshal(out, &doc) != nil || len(doc.Candidates) == 0 {
			return unavailable()
		}
		selected := doc.Candidates[0]
		for _, value := range []string{selected.CandidateID, selected.Route.Provider, selected.Route.ModelID, selected.Route.Reasoning} {
			if !approvedexec.ValidValue(value) {
				return unavailable()
			}
		}
		messages := []string{}
		for _, warning := range selected.Warnings {
			for _, state := range []string{advisory.Current, advisory.Missing, advisory.Unknown, advisory.Partial, advisory.Stale, advisory.AuthenticationError, advisory.ProviderError, advisory.Disabled} {
				if warning == advisory.ForState(state).Message {
					messages = append(messages, warning)
					break
				}
			}
		}
		reason := "Recommendation available; dispatch remains advisory."
		if !doc.UsageEnabled {
			reason = "Score-only recommendation; dispatch remains advisory."
			messages = []string{advisory.ForState(advisory.Disabled).Message}
		}
		candidate := map[string]any{"candidate_id": selected.CandidateID, "route": map[string]any{"provider": selected.Route.Provider, "model_id": selected.Route.ModelID, "reasoning": selected.Route.Reasoning}, "model_score": selected.ModelScore, "final_score": selected.FinalScore, "warnings": messages}
		return approve(reason, map[string]any{"recommendation_available": true, "candidate": candidate})
	case "model-audit":
		var doc auditDocument
		if code != 0 || json.Unmarshal(out, &doc) != nil || doc.SchemaVersion != "2.0" || !doc.Evidence.valid() {
			return companyAuditFailure(), nil
		}
		provider, model, ok := strings.Cut(doc.Candidate, ":")
		if !ok || !approvedexec.ValidValue(provider) || !approvedexec.ValidValue(model) {
			return companyAuditFailure(), nil
		}
		if expected := envOr(opts.Env, "WHICH_MODEL_CANDIDATE_ID", ""); expected != "" && expected != doc.Candidate {
			return companyAuditFailure(), nil
		}
		return companyAudit(policy, doc, opts)
	}
	return approve("Dispatch evidence is unavailable; dispatch remains advisory.", map[string]any{})
}
