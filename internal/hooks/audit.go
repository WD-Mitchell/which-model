package hooks

import "time"

// auditDocument is the private decoding boundary for F26 ExplainResult.
// Explicit fields strip unrelated input before writing audit files. Keep this
// wire view aligned with F26; hooks cannot import the CLI package that runs it.
type auditDocument struct {
	SchemaVersion string         `json:"schema_version"`
	Candidate     string         `json:"candidate"`
	Evidence      *auditEvidence `json:"evidence"`
}

type auditEvidence struct {
	Profile     string             `json:"profile"`
	ScoreInputs map[string]float64 `json:"score_inputs"`
	Band        *struct {
		Name        string   `json:"name"`
		UsedPercent *float64 `json:"used_percent"`
		Weight      *float64 `json:"weight"`
	} `json:"band,omitempty"`
	SnapshotAgeSeconds *int64 `json:"snapshot_age_seconds,omitempty"`
	Confidence         string `json:"confidence,omitempty"`
	RouteProvenance    string `json:"route_provenance"`
	ExcludedCandidates []struct {
		Route struct {
			Provider  string   `json:"provider"`
			ModelID   string   `json:"model_id"`
			Model     string   `json:"model"`
			Reasoning string   `json:"reasoning"`
			WindowIDs []string `json:"window_ids"`
		} `json:"route"`
		ReasonCode string `json:"reason_code"`
		Reason     string `json:"reason"`
	} `json:"excluded_candidates"`
	LastVerified string `json:"last_verified,omitempty"`
}

// valid rejects incomplete or out-of-contract evidence before any file is created.
func (e *auditEvidence) valid() bool {
	if e == nil || e.Profile == "" || e.ScoreInputs == nil || e.ExcludedCandidates == nil {
		return false
	}
	switch e.RouteProvenance {
	case "provider_live", "models_dev", "user_declared":
	default:
		return false
	}
	if e.Confidence != "" && e.Confidence != "live" && e.Confidence != "cached" {
		return false
	}
	if e.SnapshotAgeSeconds != nil && *e.SnapshotAgeSeconds < 0 {
		return false
	}
	if e.LastVerified != "" {
		if _, err := time.Parse(time.RFC3339, e.LastVerified); err != nil {
			return false
		}
	}
	if e.Band != nil && (e.Band.Name == "" || e.Band.UsedPercent == nil || e.Band.Weight == nil || *e.Band.UsedPercent < 0 || *e.Band.UsedPercent > 100 || *e.Band.Weight < 0) {
		return false
	}
	for _, excluded := range e.ExcludedCandidates {
		if excluded.Route.Provider == "" || excluded.Route.ModelID == "" || excluded.Route.Model == "" || excluded.Route.Reasoning == "" || excluded.Route.WindowIDs == nil || excluded.Reason == "" {
			return false
		}
		switch excluded.ReasonCode {
		case "band_gated", "no_score_row", "auth_required", "provider_error", "not_in_availability_list":
		default:
			return false
		}
	}
	return true
}
