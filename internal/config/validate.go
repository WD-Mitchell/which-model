package config

import (
	"sort"

	"github.com/shopspring/decimal"
)

func (c *Config) Validate() error {
	if c.Usage.Enabled != "" {
		if _, err := ParseUsageEnabled(string(c.Usage.Enabled)); err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: "usage.enabled", Err: err}
		}
	}
	if c.Usage.Backend != "" {
		if _, err := ParseUsageBackend(string(c.Usage.Backend)); err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: "usage.backend", Err: err}
		}
	}

	ids := make([]string, 0, len(c.Providers))
	for id := range c.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		provider := c.Providers[id]
		if id == "" {
			return &ConfigError{Kind: KindInvalidValue, Key: "providers.<empty>"}
		}
		if provider.Weight.Sign() < 0 {
			return &ConfigError{Kind: KindInvalidValue, Key: "providers." + id + ".weight"}
		}
		if provider.Weight.IsZero() {
			provider.Weight = decimal.NewFromInt(1)
			c.Providers[id] = provider
		}
		if provider.CacheTTL < 0 {
			return &ConfigError{Kind: KindInvalidValue, Key: "providers." + id + ".cache_ttl"}
		}
	}
	return nil
}
