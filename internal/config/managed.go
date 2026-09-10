package config

import (
	"sort"

	"github.com/WD-Mitchell/which-model/internal/company"
)

// Private seam for configuration tests. Production always reads the fixed OS
// origin; no Config, LoadOptions, environment key or command flag controls it.
var readCompanyPolicy = company.Load

func validateManagedAuth(policy company.Snapshot, auth AuthConfig) error {
	if !policy.Managed || auth.UseKeychain {
		return nil
	}
	if err := policy.RequireSource("managed_file"); err != nil {
		return err
	}
	return nil
}

// ValidateManaged checks authority without embedding it in mutable configuration.
// Effects also reload policy at their own boundary to observe administrator changes.
func (c *Config) ValidateManaged() error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if !policy.Managed {
		return nil
	}
	for _, key := range []string{"company", "managed_policy", "administrator_policy"} {
		if rawLookup(c.raw, key) != nil {
			return &company.Error{Origin: policy.Origin, Reason: "administrator settings cannot be supplied by user configuration"}
		}
	}
	auth, err := c.LoadAuth()
	if err != nil {
		return err
	}
	if err := validateManagedAuth(policy, auth); err != nil {
		return err
	}
	if c.Usage.Backend == UsageBackendCodexBar {
		if err := policy.RequireCodexBar(); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(c.Providers))
	for id := range c.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := c.Providers[id]
		if p.Enabled {
			if err := policy.RequireProvider(id); err != nil {
				return err
			}
		}
		if p.CredentialPath != "" {
			if err := policy.RequireSource("provider_file"); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateManagedDocument validates a proposed config-set payload in memory,
// before any file is created. Personal config-set behavior remains unchanged.
func ValidateManagedDocument(data []byte) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if !policy.Managed {
		return nil
	}
	cfg := Default()
	if err := cfg.decode(data, ""); err != nil {
		return err
	}
	return cfg.Validate()
}
