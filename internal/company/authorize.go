package company

// Authorize reloads machine policy at an operation boundary. Empty dimensions
// are irrelevant to that operation; callers cannot supply a policy or origin.
func Authorize(provider, source, capability string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if provider != "" {
		if err := s.RequireProvider(provider); err != nil {
			return err
		}
	}
	if source != "" {
		if err := s.RequireSource(source); err != nil {
			return err
		}
	}
	if capability != "" {
		return s.RequireCapability(capability)
	}
	return nil
}

// AuthorizeProviderFileWrite guards legacy native login before starting a flow
// and again before persistence. Secure managed login is supplied by #283.
func AuthorizeProviderFileWrite(provider string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if err := s.RequireProvider(provider); err != nil {
		return err
	}
	if s.Managed && s.Policy != nil && s.Policy.SecureStoreOnly {
		return s.denied("secure_store_only: provider-file login persistence")
	}
	return s.RequireSource("provider_file")
}

// AuthorizeProviderLogin permits managed flows whose persistence uses the native
// secure store. Personal flows retain their existing provider-file behavior.
func AuthorizeProviderLogin(provider string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if err := s.RequireProvider(provider); err != nil {
		return err
	}
	if s.Managed {
		return s.RequireSource("keychain")
	}
	return nil
}
