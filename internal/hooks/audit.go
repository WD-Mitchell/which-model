package hooks

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
		Name        string  `json:"name"`
		UsedPercent float64 `json:"used_percent"`
		Weight      float64 `json:"weight"`
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
