package config

// AuthConfig controls storage for credentials created by which-model.
type AuthConfig struct {
	UseKeychain    bool `toml:"use_keychain" json:"use_keychain"`
	NativeKeychain bool `toml:"native_keychain" json:"native_keychain"`
}

type authConfigTOML struct {
	UseKeychain    *bool `toml:"use_keychain"`
	NativeKeychain *bool `toml:"native_keychain"`
}

// DefaultAuthConfig returns the credential-storage defaults.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{UseKeychain: true}
}

// LoadAuth decodes [auth] with per-key defaults.
func (c *Config) LoadAuth() (AuthConfig, error) {
	auth := DefaultAuthConfig()
	var mirror authConfigTOML
	if err := c.UnmarshalKey("auth", &mirror); err != nil {
		return AuthConfig{}, err
	}
	if mirror.UseKeychain != nil {
		auth.UseKeychain = *mirror.UseKeychain
	}
	if mirror.NativeKeychain != nil {
		auth.NativeKeychain = *mirror.NativeKeychain
	}
	return auth, nil
}

// SetAuth writes the complete [auth] section into the raw document.
func (c *Config) SetAuth(auth AuthConfig) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	if err := validateManagedAuth(policy, auth); err != nil {
		return err
	}
	c.setRaw("auth", map[string]any{
		"use_keychain":    auth.UseKeychain,
		"native_keychain": auth.NativeKeychain,
	})
	return nil
}
