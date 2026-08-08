//go:build nousage

package toggle

import (
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func makeCfg(enabled config.UsageEnabled, enabledProviders ...string) *config.Config {
	providers := make(map[string]config.ProviderConfig)
	for _, name := range enabledProviders {
		providers[name] = config.ProviderConfig{Enabled: true}
	}
	return &config.Config{Usage: config.UsageConfig{Enabled: enabled}, Providers: providers}
}

func TestCompiledNouseage(t *testing.T) {
	if Compiled {
		t.Error("Compiled = true under -tags nousage, want false")
	}
}

func TestResolveUsageEnabledNouseage(t *testing.T) {
	tests := []struct {
		name        string
		flagNoUsage bool
		cfg         *config.Config
		wantEnabled bool
		wantReason  string
	}{
		{
			name:        "flag ignored, nil cfg",
			flagNoUsage: true,
			cfg:         nil,
			wantEnabled: false,
			wantReason:  ReasonCompiledOut,
		},
		{
			name:        "config ignored even when it would enable",
			flagNoUsage: false,
			cfg:         makeCfg(config.UsageTrue, "claude"),
			wantEnabled: false,
			wantReason:  ReasonCompiledOut,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, reason := ResolveUsageEnabled(tt.flagNoUsage, tt.cfg)
			if enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", enabled, tt.wantEnabled)
			}
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestReasonCompiledOutValue(t *testing.T) {
	if ReasonCompiledOut != "compiled_out" {
		t.Errorf("ReasonCompiledOut = %q, want %q", ReasonCompiledOut, "compiled_out")
	}
}
