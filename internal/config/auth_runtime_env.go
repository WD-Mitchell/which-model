package config

// isAuthRuntimeEnv identifies inputs owned by credential and login layers.
// These exact names are not configuration overrides; typos still fail validation.
func isAuthRuntimeEnv(name string) bool {
	return name == "WHICH_MODEL_CLAUDE_OAUTH_TOKEN" || name == "WHICH_MODEL_NONINTERACTIVE"
}
