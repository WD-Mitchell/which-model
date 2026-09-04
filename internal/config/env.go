package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Non-config WHICH_MODEL inputs have separate owners; retain strict rejection
// for every other unknown prefixed name.
var runtimeEnvKeys = map[string]bool{
	"WHICH_MODEL_CONFIG":           true,
	"WHICH_MODEL_TASK_PROFILE":     true,
	"WHICH_MODEL_CANDIDATE_ID":     true,
	"WHICH_MODEL_DISPATCHED_MODEL": true,
}

var envKeys = map[string]bool{
	"enabled":                 true,
	"backend":                 true,
	"priority":                true,
	"weight":                  true,
	"cache_ttl":               true,
	"credential_path":         true,
	"trusted_fallback_origin": true,
	"default":                 true,
	"default_profile":         true,
	"tier1_share":             true,
	"tier2_share":             true,
	"direction":               true,
	"gate_above_used_percent": true,
	"normalizer":              true,
	"aggregator":              true,
	"raw_csv_path":            true,
	"scores_csv_path":         true,
	"provider_config_path":    true,
	"benchmark_config_path":   true,
	"warn_on_stale_scores":    true,
	"schedule":                true,
	"timezone":                true,
	"mode":                    true,
	"auto_merge":              true,
	"merge_method":            true,
	"commit_message":          true,
	"pr_title":                true,
	"run_tests":               true,
	"color":                   true,
	"timestamps":              true,
	"identity_default":        true,
}

// envSections maps each generic config section to the env-addressable scalar
// keys it owns (F01 CONTRACTS §3 closed vocabulary). The flat envKeys set
// only decides the longest-suffix key match; this table decides whether the
// resolved section actually owns that key, so a valid key under the wrong
// section fails eagerly (D14) instead of being stored where no
// UnmarshalKey caller will ever decode it.
var envSections = map[string]map[string]bool{
	"usage":           {"enabled": true, "backend": true},
	"strategy":        {"default": true, "default_profile": true, "tier1_share": true, "tier2_share": true},
	"bands":           {"direction": true, "gate_above_used_percent": true},
	"scoring":         {"normalizer": true, "aggregator": true},
	"catalog":         {"raw_csv_path": true, "scores_csv_path": true, "provider_config_path": true, "benchmark_config_path": true, "cache_ttl": true, "warn_on_stale_scores": true},
	"catalog.publish": {"enabled": true, "schedule": true, "timezone": true, "mode": true, "auto_merge": true, "merge_method": true, "commit_message": true, "pr_title": true, "run_tests": true},
	"output":          {"color": true, "timestamps": true, "identity_default": true},
}

// providerEnvKeys are the env-addressable keys of every [providers.<id>]
// table; the section is dynamic (one per provider id), so ownership is
// checked against this fixed set.
var providerEnvKeys = map[string]bool{
	"enabled":                 true,
	"priority":                true,
	"weight":                  true,
	"cache_ttl":               true,
	"credential_path":         true,
	"trusted_fallback_origin": true,
}

func envSectionOwns(sectionPath, key string) bool {
	if strings.HasPrefix(sectionPath, "providers.") {
		return providerEnvKeys[key]
	}
	return envSections[sectionPath][key]
}

var EnvKeys = envKeys

func ApplyEnv(c *Config, getenv func(string) string, environ []string) error {
	lookup := getenv
	if lookup == nil {
		lookup = os.Getenv
	}
	envs := environ
	if envs == nil {
		envs = os.Environ()
	}
	if c.env == nil {
		c.env = make(map[string]string)
	}
	if c.Providers == nil {
		c.Providers = make(map[string]ProviderConfig)
	}

	for _, entry := range envs {
		equal := strings.IndexByte(entry, '=')
		if equal < 0 {
			continue
		}
		name := entry[:equal]
		if !strings.HasPrefix(name, "WHICH_MODEL_") || runtimeEnvKeys[name] {
			continue
		}
		rest := strings.ToLower(strings.TrimPrefix(name, "WHICH_MODEL_"))
		suffix, ok := envSuffix(rest)
		if !ok {
			return &ConfigError{Kind: KindInvalidValue, Key: name}
		}
		sectionPrefix := strings.TrimSuffix(rest[:len(rest)-len(suffix)], "_")
		sectionPath := strings.ReplaceAll(sectionPrefix, "_", ".")
		if sectionPath == "" {
			return &ConfigError{Kind: KindInvalidValue, Key: name}
		}
		if !envSectionOwns(sectionPath, suffix) {
			return &ConfigError{Kind: KindInvalidValue, Key: name}
		}
		value := lookup(name)
		if sectionPath == "usage" {
			switch suffix {
			case "enabled":
				parsed, err := ParseUsageEnabled(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				c.Usage.Enabled = parsed
				continue
			case "backend":
				parsed, err := ParseUsageBackend(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				c.Usage.Backend = parsed
				continue
			}
		}

		if strings.HasPrefix(sectionPath, "providers.") {
			providerID := strings.TrimPrefix(sectionPath, "providers.")
			providerID = strings.ReplaceAll(providerID, ".", "-")
			if providerID == "" {
				return &ConfigError{Kind: KindInvalidValue, Key: name}
			}
			provider := c.Providers[providerID]
			switch suffix {
			case "enabled":
				parsed, err := strconv.ParseBool(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				provider.Enabled = parsed
			case "priority":
				parsed, err := strconv.Atoi(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				provider.Priority = parsed
			case "weight":
				parsed, err := decimal.NewFromString(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				provider.Weight = parsed
			case "cache_ttl":
				parsed, err := time.ParseDuration(value)
				if err != nil {
					return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
				}
				provider.CacheTTL = parsed
			case "credential_path":
				provider.CredentialPath = value
			case "trusted_fallback_origin":
				provider.TrustedFallbackOrigin = value
			}
			c.Providers[providerID] = provider
			continue
		}

		c.env[sectionPath+"."+suffix] = value
	}
	return nil
}

func envSuffix(rest string) (string, bool) {
	for i := range len(rest) {
		if i != 0 && rest[i-1] != '_' {
			continue
		}
		if envKeys[rest[i:]] {
			return rest[i:], true
		}
	}
	return "", false
}
