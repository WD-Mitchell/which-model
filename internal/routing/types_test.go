package routing

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProvenanceValues(t *testing.T) {
	tests := []struct {
		name string
		got  Provenance
		want string
	}{
		{"provider live", ProvenanceProviderLive, "provider_live"},
		{"models dev", ProvenanceModelsDev, "models_dev"},
		{"user declared", ProvenanceUserDeclared, "user_declared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.got); got != tt.want {
				t.Fatalf("Provenance = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRouteJSONRoundTrip(t *testing.T) {
	original := Route{
		Provider:   "claude",
		ModelID:    "claude-opus-4-5-20251101",
		Model:      "Claude Opus 5",
		Reasoning:  "default",
		WindowIDs:  []string{"5h", "sevenDayOpus"},
		Provenance: ProvenanceProviderLive,
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("json.Unmarshal object: %v", err)
	}
	wantKeys := map[string]struct{}{
		"provider": {}, "model_id": {}, "model": {}, "reasoning": {}, "window_ids": {}, "provenance": {},
	}
	if !reflect.DeepEqual(mapKeys(object), wantKeys) {
		t.Fatalf("JSON keys = %v, want %v", mapKeys(object), wantKeys)
	}

	var got Route
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal route: %v", err)
	}
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round trip = %#v, want %#v", got, original)
	}
	if !reflect.DeepEqual(got.WindowIDs, original.WindowIDs) {
		t.Fatalf("WindowIDs = %#v, want %#v", got.WindowIDs, original.WindowIDs)
	}
}

func TestRouteJSONNullWindowIDs(t *testing.T) {
	var route Route
	if err := json.Unmarshal([]byte(`{"provider":"claude","model_id":"m","model":"M","reasoning":"default","window_ids":null,"provenance":"models_dev"}`), &route); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if route.WindowIDs != nil {
		t.Fatalf("WindowIDs = %#v, want nil", route.WindowIDs)
	}
}

func mapKeys(values map[string]json.RawMessage) map[string]struct{} {
	keys := make(map[string]struct{}, len(values))
	for key := range values {
		keys[key] = struct{}{}
	}
	return keys
}
