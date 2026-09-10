package hooks

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"path/filepath"
	"strings"
)

var readPrivacyPolicy = company.Load

func companyAudit(policy company.Snapshot, doc auditDocument, opts Options) ([]byte, error) {
	ctl := privacy.Controller{Policy: policy}
	layout, err := privacy.DefaultLayout()
	if opts.companyStateDir != "" {
		layout = privacy.Layout{StateDir: opts.companyStateDir}
	}
	if err != nil {
		return companyAuditFailure(), nil
	}
	layout.ProjectRoot = opts.RepoRoot
	if _, err := ctl.Maintain(layout, false, []privacy.Category{privacy.Audit}); err != nil {
		return companyAuditFailure(), nil
	}
	if policy.Policy.Retention.AuditRecordsDays == 0 {
		return MarshalEnvelope(Envelope{Decision: "approve", Reason: "Company audit retention is zero; no evidence was recorded. Dispatch remains advisory.", HookSpecificOutput: map[string]any{"audit_recorded": false, "audit_status": "disabled"}}), nil
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return companyAuditFailure(), nil
	}
	path := filepath.Join(layout.StateDir, "audit", "evidence.jsonl")
	if err := ctl.Append(path, privacy.Audit, data); err != nil {
		return companyAuditFailure(), nil
	}
	mismatch := false
	_, modelID, _ := strings.Cut(doc.Candidate, ":")
	if dispatched := envOr(opts.Env, "WHICH_MODEL_DISPATCHED_MODEL", ""); dispatched != "" && dispatched != modelID {
		mismatch = true
		data, err = json.Marshal(map[string]any{"candidate": doc.Candidate, "dispatched_model": dispatched, "route_model_id": modelID, "evidence": doc.Evidence})
		if err != nil {
			return companyAuditFailure(), nil
		}
		if err := ctl.Append(filepath.Join(layout.StateDir, "audit", "mismatches.jsonl"), privacy.Audit, data); err != nil {
			return companyAuditFailure(), nil
		}
	}
	return MarshalEnvelope(Envelope{Decision: "approve", Reason: "dispatch evidence recorded", HookSpecificOutput: map[string]any{"audit_recorded": true, "evidence_logged": "managed audit store", "mismatch": mismatch}}), nil
}

func companyAuditFailure() []byte {
	return MarshalEnvelope(Envelope{Decision: "approve", Reason: "company audit persistence failed; dispatch remains advisory", HookSpecificOutput: map[string]any{"audit_recorded": false}})
}
