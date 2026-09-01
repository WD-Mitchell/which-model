package config

// AuthConfig controls storage for credentials created by which-model.
type AuthConfig struct {
	UseKeychain bool `toml:"use_keychain" json:"use_keychain"`
}

type authConfigTOML struct {
	UseKeychain *bool `toml:"use_keychain"`
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
	return auth, nil
}

// SetAuth writes the complete [auth] section into the raw document.
func (c *Config) SetAuth(auth AuthConfig) error {
	c.setRaw("auth", map[string]any{
		"use_keychain": auth.UseKeychain,
	})
	return nil
}
