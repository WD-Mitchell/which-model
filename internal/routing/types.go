// Package routing holds the canonical route types (specs/global/CONTRACTS.md
// §3.1; placement recorded in specs/DEFERRED.md D11). F18's ProduceRoutes and
// persistence layers build on these declarations.
package routing

// Provenance records how a Route was established.
type Provenance string

const (
	ProvenanceProviderLive Provenance = "provider_live"
	ProvenanceModelsDev    Provenance = "models_dev"
	ProvenanceUserDeclared Provenance = "user_declared"
)

// Route is one provider-native model ↔ catalog identity join
// (specs/global/CONTRACTS.md §3.1).
type Route struct {
	Provider   string     `json:"provider"`
	ModelID    string     `json:"model_id"`
	Model      string     `json:"model"`
	Reasoning  string     `json:"reasoning"`
	WindowIDs  []string   `json:"window_ids"`
	Provenance Provenance `json:"provenance"`
}
