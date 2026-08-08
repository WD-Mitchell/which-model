package whichmodel

import "github.com/WD-Mitchell/which-model/internal/config"

// OutputConfig is F22's [output] section schema (F01 DECISION B correction;
// SPEC §12). F03 renderers consume the colour decision.
type OutputConfig struct {
	Color           string `toml:"color"`            // "auto" | "always" | "never"; default "auto"
	Timestamps      string `toml:"timestamps"`       // "rfc3339" | "none"; default "rfc3339"
	IdentityDefault bool   `toml:"identity_default"` // default false
}

// DefaultOutputConfig returns the [output] defaults.
func DefaultOutputConfig() OutputConfig {
	return OutputConfig{Color: "auto", Timestamps: "rfc3339", IdentityDefault: false}
}

// loadOutputConfig decodes the [output] section into the defaults via
// cfg.UnmarshalKey; unknown keys surface as a F01 ConfigError.
func loadOutputConfig(cfg *config.Config) (OutputConfig, error) {
	out := DefaultOutputConfig()
	if err := cfg.UnmarshalKey("output", &out); err != nil {
		return out, err
	}
	return out, nil
}
