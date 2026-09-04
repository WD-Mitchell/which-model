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
	Backend UsageBackend `toml:"backend"`
}

// ProviderAccount is one named credential a provider can authenticate with.
//
// It records a REFERENCE, never a secret: Ref is the env var, file path or
// keychain service the credential lives in, mirroring how usage descriptors
// already resolve auth (internal/usage AuthSource). config.toml is
// world-readable in practice, so a token in it would be a liability.
type ProviderAccount struct {
	Name string `toml:"name"` // display name, e.g. "Work"
	Kind string `toml:"kind"` // oauth | cookie | token
	Ref  string `toml:"ref"`  // env var, file path, or keychain service
}

type ProviderConfig struct {
	Enabled               bool              `toml:"enabled"`
	Accounts              []ProviderAccount `toml:"accounts"`
	Priority              int               `toml:"priority"`
	Weight                decimal.Decimal   `toml:"weight"`
	CacheTTL              time.Duration     `toml:"cache_ttl"`
	SourcePreference      []string          `toml:"source_preference"`
	CredentialPath        string            `toml:"credential_path"`
	TrustedFallbackOrigin string            `toml:"trusted_fallback_origin"`
}

func Default() *Config {
	return &Config{
		Usage:     UsageConfig{Enabled: UsageAuto, Backend: UsageBackendOff},
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

// Clone preserves typed values and deferred environment overrides without TOML coercion.
func (c *Config) Clone() *Config {
	next := *c
	if c.raw != nil {
		next.raw = deepCopyRaw(c.raw).(map[string]any)
	}
	next.env = make(map[string]string, len(c.env))
	for k, v := range c.env {
		next.env[k] = v
	}
	next.Providers = make(map[string]ProviderConfig, len(c.Providers))
	for k, v := range c.Providers {
		v.Accounts = append([]ProviderAccount(nil), v.Accounts...)
		v.SourcePreference = append([]string(nil), v.SourcePreference...)
		next.Providers[k] = v
	}
	return &next
}
