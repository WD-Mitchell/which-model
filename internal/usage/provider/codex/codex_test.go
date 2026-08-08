//go:build !nousage

package codex

import (
	"context"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// TestDescriptorRegistered pins the F16 descriptor (CONTRACTS §2) field for
// field: the registry lookup succeeds, and every constant/table matches.
func TestDescriptorRegistered(t *testing.T) {
	d, err := usage.Get("codex")
	if err != nil {
		t.Fatalf("usage.Get(\"codex\") failed: %v", err)
	}
	if d.ID != "codex" {
		t.Errorf("ID = %q, want %q", d.ID, "codex")
	}
	if d.DisplayName != "Codex" {
		t.Errorf("DisplayName = %q, want %q", d.DisplayName, "Codex")
	}
	if d.Kind != usage.KindSubscription {
		t.Errorf("Kind = %v, want %v", d.Kind, usage.KindSubscription)
	}
	if d.Tier != 1 {
		t.Errorf("Tier = %d, want 1", d.Tier)
	}
	if d.Timeout != 15*time.Second {
		t.Errorf("Timeout = %v, want 15s", d.Timeout)
	}
	if d.CacheTTL != 60*time.Second {
		t.Errorf("CacheTTL = %v, want 60s", d.CacheTTL)
	}

	wantIDs := []string{"5h", "weekly", "credits"}
	if len(d.Windows) != len(wantIDs) {
		t.Fatalf("len(Windows) = %d, want %d", len(d.Windows), len(wantIDs))
	}
	for i, id := range wantIDs {
		if d.Windows[i].ID != id {
			t.Errorf("Windows[%d].ID = %q, want %q", i, d.Windows[i].ID, id)
		}
		if !d.Windows[i].Optional {
			t.Errorf("Windows[%d] (%s) Optional = false, want true", i, id)
		}
	}
	if d.Windows[0].Label != "primary window" || d.Windows[1].Label != "secondary window" || d.Windows[2].Label != "credits" {
		t.Errorf("window labels = %q/%q/%q, want %q/%q/%q",
			d.Windows[0].Label, d.Windows[1].Label, d.Windows[2].Label,
			"primary window", "secondary window", "credits")
	}
	if d.Windows[0].Unit != usage.UnitPercent || d.Windows[1].Unit != usage.UnitPercent {
		t.Errorf("5h/weekly units = %q/%q, want percent/percent", d.Windows[0].Unit, d.Windows[1].Unit)
	}
	if d.Windows[2].Unit != usage.UnitCredits {
		t.Errorf("credits unit = %q, want %q", d.Windows[2].Unit, usage.UnitCredits)
	}
}

// TestDescriptorAuthChain pins the six tolerated token shapes over
// [$CODEX_HOME, ~/.codex]/auth.json, in order (SPEC §2.4).
func TestDescriptorAuthChain(t *testing.T) {
	d, err := usage.Get("codex")
	if err != nil {
		t.Fatalf("usage.Get(\"codex\") failed: %v", err)
	}
	wantPaths := []string{"$CODEX_HOME/auth.json", "~/.codex/auth.json"}
	wantJSONPaths := []string{
		"tokens.access_token",
		"tokens.accessToken",
		"auth.access_token",
		"auth.accessToken",
		"access_token",
		"accessToken",
	}
	if len(d.Auth) != 6 {
		t.Fatalf("len(Auth) = %d, want 6", len(d.Auth))
	}
	for i, src := range d.Auth {
		if src.Kind != usage.AuthFile {
			t.Errorf("Auth[%d].Kind = %v, want AuthFile", i, src.Kind)
		}
		if len(src.FilePaths) != 2 || src.FilePaths[0] != wantPaths[0] || src.FilePaths[1] != wantPaths[1] {
			t.Errorf("Auth[%d].FilePaths = %v, want %v", i, src.FilePaths, wantPaths)
		}
		if src.JSONPath != wantJSONPaths[i] {
			t.Errorf("Auth[%d].JSONPath = %q, want %q", i, src.JSONPath, wantJSONPaths[i])
		}
		if len(src.ExtraPaths) != 0 {
			t.Errorf("Auth[%d].ExtraPaths = %v, want none (SPEC D1)", i, src.ExtraPaths)
		}
	}
}

func TestUsageURL(t *testing.T) {
	if UsageURL != "https://chatgpt.com/backend-api/wham/usage" {
		t.Errorf("UsageURL = %q, want %q", UsageURL, "https://chatgpt.com/backend-api/wham/usage")
	}
}

func TestFallbackStatuses(t *testing.T) {
	want := map[int]bool{404: true, 405: true, 410: true, 501: true}
	if len(FallbackStatuses) != len(want) {
		t.Fatalf("len(FallbackStatuses) = %d, want %d", len(FallbackStatuses), len(want))
	}
	for status := 100; status <= 599; status++ {
		if FallbackStatuses[status] != want[status] {
			t.Errorf("FallbackStatuses[%d] = %v, want %v", status, FallbackStatuses[status], want[status])
		}
	}
}

func TestTrustedOriginContext(t *testing.T) {
	ctx := context.Background()
	if got := TrustedOriginFrom(ctx); got != "" {
		t.Errorf("TrustedOriginFrom(empty ctx) = %q, want \"\"", got)
	}
	ctx = WithTrustedOrigin(ctx, "https://trusted.example")
	if got := TrustedOriginFrom(ctx); got != "https://trusted.example" {
		t.Errorf("TrustedOriginFrom = %q, want %q", got, "https://trusted.example")
	}
}
