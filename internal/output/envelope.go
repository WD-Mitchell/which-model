package output

// OutputEnvelope is the canonical envelope carried by every --json document.
// Defined in specs/global/CONTRACTS.md §6; F03 owns the Go type.
type OutputEnvelope struct {
	SchemaVersion       string `json:"schema_version"` // "2.0"
	UsageEnabled        bool   `json:"usage_enabled"`
	UsageDisabledReason string `json:"usage_disabled_reason,omitempty"` // flag|config|compiled_out|no_providers_enabled
}

// SchemaVersion is the current JSON schema version (specs/global/CONTRACTS.md §7).
const SchemaVersion = "2.0"
