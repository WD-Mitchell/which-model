package config

import (
	"strings"
	"testing"
)

func TestAuthConfigDefaultAndRoundTrip(t *testing.T) {
	cfg := loadFixture(t, "")
	got, err := cfg.LoadAuth()
	if err != nil {
		t.Fatal(err)
	}
	if !got.UseKeychain {
		t.Fatal("default use_keychain = false, want true")
	}

	cfg = loadFixture(t, "[auth]\nuse_keychain = false\n")
	got, err = cfg.LoadAuth()
	if err != nil {
		t.Fatal(err)
	}
	if got.UseKeychain {
		t.Fatal("configured use_keychain = true, want false")
	}

	if err := cfg.SetAuth(AuthConfig{UseKeychain: true}); err != nil {
		t.Fatal(err)
	}
	data, err := cfg.MarshalTOML()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "[auth]") || !strings.Contains(text, "use_keychain = true") {
		t.Fatalf("marshaled auth config = %q", text)
	}
}

func TestAuthConfigRejectsUnknownKey(t *testing.T) {
	cfg := loadFixture(t, "[auth]\nunknown = true\n")
	if _, err := cfg.LoadAuth(); err == nil || !strings.Contains(err.Error(), "auth.unknown") {
		t.Fatalf("LoadAuth error = %v", err)
	}
}
