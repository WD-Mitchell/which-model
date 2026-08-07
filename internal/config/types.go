package config

import (
	"time"

	"github.com/shopspring/decimal"
)

type Config struct {
	Usage     UsageConfig
	Providers map[string]ProviderConfig
	raw       map[string]any
	env       map[string]string
}

type UsageConfig struct {
	Enabled UsageEnabled `toml:"enabled"`
}

type ProviderConfig struct {
	Enabled               bool            `toml:"enabled"`
	Priority              int             `toml:"priority"`
	Weight                decimal.Decimal `toml:"weight"`
	CacheTTL              time.Duration   `toml:"cache_ttl"`
	SourcePreference      []string        `toml:"source_preference"`
	CredentialPath        string          `toml:"credential_path"`
	TrustedFallbackOrigin string          `toml:"trusted_fallback_origin"`
}

func Default() *Config {
	return &Config{
		Usage:     UsageConfig{Enabled: UsageAuto},
		Providers: make(map[string]ProviderConfig),
	}
}

type ErrorKind int

const (
	KindNotFound ErrorKind = iota
	KindUnreadable
	KindInvalidTOML
	KindInvalidValue
)

type ConfigError struct {
	Kind ErrorKind
	Path string
	Key  string
	Err  error
}

func (e *ConfigError) Error() string {
	var message string
	switch e.Kind {
	case KindNotFound:
		message = "file not found"
	case KindUnreadable:
		message = "unreadable"
		if e.Err != nil {
			message += ": " + e.Err.Error()
		}
	case KindInvalidTOML:
		message = "invalid TOML"
		if e.Err != nil {
			message += ": " + e.Err.Error()
		}
	case KindInvalidValue:
		message = "invalid value for " + e.Key
		if e.Err != nil {
			message += ": " + e.Err.Error()
		} else {
			message += ": " + e.Key
		}
	default:
		message = "invalid configuration"
		if e.Err != nil {
			message += ": " + e.Err.Error()
		}
	}
	if e.Path == "" {
		return "config: " + message
	}
	return "config: " + e.Path + ": " + message
}

func (e *ConfigError) Unwrap() error { return e.Err }

func (e *ConfigError) ExitCode() int { return 2 }
