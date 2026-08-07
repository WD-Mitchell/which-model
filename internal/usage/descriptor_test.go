package usage

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func codexStyleDescriptor() Descriptor {
	return Descriptor{
		ID:          "codex",
		DisplayName: "Codex",
		Kind:        KindSubscription,
		Tier:        1,
		Auth: []AuthSource{
			{
				Kind:       AuthFile,
				FilePaths:  []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"},
				JSONPath:   "tokens.access_token",
				ExtraPaths: map[string]string{"account_id": "tokens.account_id"},
			},
		},
		Windows: []WindowSpec{
			{ID: "primary", Label: "Primary", Unit: UnitPercent},
			{ID: "secondary", Label: "Secondary", Unit: UnitPercent},
			{ID: "credits", Label: "Credits", Unit: UnitUSD, Optional: true},
		},
		Timeout:  15 * time.Second,
		CacheTTL: 300 * time.Second,
	}
}

func TestDescriptorShape(t *testing.T) {
	d := codexStyleDescriptor()
	if d.ID != "codex" || d.Kind != KindSubscription {
		t.Errorf("descriptor identity = (%q, %v), want (codex, KindSubscription)", d.ID, d.Kind)
	}
	if !d.Windows[2].Optional {
		t.Error("credits window must be Optional")
	}
	d.Windows[0].ModelScope = []string{"gpt-5-codex"}
	if d.Windows[0].ModelScope[0] != "gpt-5-codex" {
		t.Errorf("ModelScope round-trip failed: %v", d.Windows[0].ModelScope)
	}
	if d.Auth[0].JSONPath != "tokens.access_token" {
		t.Errorf("JSONPath = %q", d.Auth[0].JSONPath)
	}
	if d.Auth[0].ExtraPaths["account_id"] != "tokens.account_id" {
		t.Errorf("ExtraPaths[account_id] = %q", d.Auth[0].ExtraPaths["account_id"])
	}
	d.Fetch = func(ctx context.Context, cred Credential, client *http.Client) (Snapshot, error) {
		return Snapshot{Provider: "codex"}, nil
	}
	if d.Timeout != 15*time.Second || d.CacheTTL != 300*time.Second || !d.LastVerified.IsZero() {
		t.Errorf("durations = (%v, %v, %v), want (15s, 300s, zero)", d.Timeout, d.CacheTTL, d.LastVerified)
	}
}
