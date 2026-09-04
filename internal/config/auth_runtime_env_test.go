package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadLeavesDocumentedAuthRuntimeEnvironmentToItsOwner(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"WHICH_MODEL_CLAUDE_OAUTH_TOKEN", "synthetic-native-auth-canary"},
		{"WHICH_MODEL_NONINTERACTIVE", "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.name, tc.value)
			path := writeFile(t, t.TempDir(), "config.toml", "[usage]\nbackend = \"native\"\n[providers.claude]\nenabled = true\n")
			cfg, err := Load(LoadOptions{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			data, err := cfg.MarshalTOML()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), tc.name) || strings.Contains(string(data), "synthetic-native-auth-canary") {
				t.Fatal("auth runtime input leaked into saved configuration")
			}
			if os.Getenv(tc.name) != tc.value {
				t.Fatal("config loading changed runtime input")
			}
		})
	}
	t.Setenv("WHICH_MODEL_CLAUDE_OAUTH_TOKENS", "synthetic-typo-canary")
	path := writeFile(t, t.TempDir(), "config.toml", "")
	if _, err := Load(LoadOptions{Path: path}); err == nil {
		t.Fatal("runtime-looking typo was accepted")
	}
}
