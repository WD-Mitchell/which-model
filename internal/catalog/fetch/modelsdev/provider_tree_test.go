package modelsdev

import (
	"reflect"
	"testing"
)

// The production https://models.dev/api.json shape (verified live 2026-08-19):
// an object keyed by provider slug; models keyed by id; reasoning_options an
// ARRAY of typed entries where only type=="effort" carries levels.
func TestParseProvidersTreeShape(t *testing.T) {
	body := []byte(`{
		"anthropic": {"id": "anthropic", "models": {
			"claude-opus-5": {"id": "claude-opus-5", "name": "Claude Opus 5 (latest)", "family": "claude",
				"reasoning": true,
				"reasoning_options": [
					{"type": "toggle", "values": null},
					{"type": "effort", "values": ["low", "medium", "high"]}
				]},
			"claude-haiku-1": {"id": "claude-haiku-1", "name": "Claude Haiku 1", "status": "deprecated"}
		}},
		"openai": {"id": "openai", "models": {
			"gpt-4o": {"id": "gpt-4o", "name": "GPT-4o"}
		}}
	}`)
	got, err := parseProviders(body)
	if err != nil {
		t.Fatalf("parseProviders: %v", err)
	}
	want := []ProviderModel{
		{Provider: "anthropic", ModelID: "claude-opus-5", Name: "Claude Opus 5",
			BaseModel: "claude", Reasoning: true, EffortLevels: []string{"high", "low", "medium"}},
		{Provider: "openai", ModelID: "gpt-4o", Name: "GPT-4o"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseProviders =\n%+v\nwant\n%+v", got, want)
	}
}
