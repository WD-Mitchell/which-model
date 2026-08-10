//go:build !nousage

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
	return &config.Config{Usage: config.UsageConfig{Enabled: enabled, Backend: config.UsageBackendNative}, Providers: providers}
}

func TestReasonConstants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{ReasonFlag, "flag"},
		{ReasonConfig, "config"},
		{ReasonBackendOff, "backend_off"},
		{ReasonCompiledOut, "compiled_out"},
		{ReasonNoProvidersEnabled, "no_providers_enabled"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("reason constant = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestCompiled(t *testing.T) {
	if !Compiled {
		t.Error("Compiled = false in the default build, want true")
	}
}

func TestResolveUsageEnabled(t *testing.T) {
	tests := []struct {
		name        string
		flagNoUsage bool
		cfg         *config.Config
		wantEnabled bool
		wantReason  string
	}{
		{
			name:        "flag beats false config",
			flagNoUsage: true,
			cfg:         makeCfg(config.UsageFalse),
			wantEnabled: false,
			wantReason:  ReasonFlag,
		},
		{
			name:        "flag beats true config with providers",
			flagNoUsage: true,
			cfg:         makeCfg(config.UsageTrue, "claude", "codex"),
			wantEnabled: false,
			wantReason:  ReasonFlag,
		},
		{
			name:        "flag beats auto config with provider",
			flagNoUsage: true,
			cfg:         makeCfg(config.UsageAuto, "claude"),
			wantEnabled: false,
			wantReason:  ReasonFlag,
		},
		{
			name:        "config false, no providers",
			cfg:         makeCfg(config.UsageFalse),
			wantEnabled: false,
			wantReason:  ReasonConfig,
		},
		{
			name:        "config false beats providers",
			cfg:         makeCfg(config.UsageFalse, "claude"),
			wantEnabled: false,
			wantReason:  ReasonConfig,
		},
		{
			name:        "auto, no enabled providers",
			cfg:         makeCfg(config.UsageAuto),
			wantEnabled: false,
			wantReason:  ReasonNoProvidersEnabled,
		},
		{
			name:        "auto, one enabled provider",
			cfg:         makeCfg(config.UsageAuto, "claude"),
			wantEnabled: true,
			wantReason:  "",
		},
		{
			name:        "auto, two of three providers enabled",
			cfg:         makeCfg(config.UsageAuto, "claude", "codex"),
			wantEnabled: true,
			wantReason:  "",
		},
		{
			name:        "true, two enabled providers",
			cfg:         makeCfg(config.UsageTrue, "claude", "codex"),
			wantEnabled: true,
			wantReason:  "",
		},
		{
			name:        "true, zero enabled providers (strict pair)",
			cfg:         makeCfg(config.UsageTrue),
			wantEnabled: false,
			wantReason:  ReasonNoProvidersEnabled,
		},
		{
			name:        "flag short-circuits nil config",
			flagNoUsage: true,
			cfg:         nil,
			wantEnabled: false,
			wantReason:  ReasonFlag,
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
func TestResolveUsageBackendOff(t *testing.T) {
	cfg := makeCfg(config.UsageAuto, "claude")
	cfg.Usage.Backend = config.UsageBackendOff
	enabled, reason := ResolveUsageEnabled(false, cfg)
	if enabled || reason != ReasonBackendOff {
		t.Fatalf("ResolveUsageEnabled() = %v, %q; want false, %q", enabled, reason, ReasonBackendOff)
	}
}
