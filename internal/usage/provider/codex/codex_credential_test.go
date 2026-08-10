//go:build !nousage

package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestLoadCredential ports the loadCodexCredential table from F16-T3
// (codex.mjs:52-61).
func TestLoadCredential(t *testing.T) {
	cases := []struct {
		name      string
		auth      string // auth.json content; "" = file absent
		config    string // config.toml content; "" = file absent
		wantToken string
		wantAcct  string
		wantBase  string
		wantErr   string // "code: message" or ""
	}{
		{
			name:      "tokens snake",
			auth:      `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct-synthetic"}}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct-synthetic",
		},
		{
			name:      "tokens camel",
			auth:      `{"tokens":{"accessToken":"canary-secret-token-123","accountId":"acct-synthetic"}}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct-synthetic",
		},
		{
			name:      "auth object + root account",
			auth:      `{"auth":{"access_token":"canary-secret-token-123"},"account_id":"acct"}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
		},
		{
			name:      "flat value + chatgpt_account_id",
			auth:      `{"access_token":"canary-secret-token-123","chatgpt_account_id":"acct"}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
		},
		{
			name:      "base_url on auth.json",
			auth:      `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct"},"base_url":"https://trusted.example/v1"}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
			wantBase:  "https://trusted.example/v1",
		},
		{
			name:      "base_url from config.toml via ParseConfig",
			auth:      `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct"}}`,
			config:    "model_provider = \"trusted\"\n[model_providers.trusted]\nbase_url = \"https://trusted.example/v1\"",
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
			wantBase:  "https://trusted.example/v1",
		},
		{
			name:    "auth.json missing",
			wantErr: "credential_file: Codex credentials were not found; sign in with Codex first.",
		},
		{
			name:    "auth.json malformed",
			auth:    `{bad`,
			wantErr: "credential_json: The credential file is not valid JSON.",
		},
		{
			name:    "auth.json empty object",
			auth:    `{}`,
			wantErr: "unsafe_credential: The Codex access token is missing or unsafe.",
		},
		{
			name:    "account id with space",
			auth:    `{"tokens":{"access_token":"canary-secret-token-123","account_id":"bad id"}}`,
			wantErr: "unsafe_credential: The ChatGPT account identifier is missing or unsafe.",
		},
		{
			name:      "config.toml missing silently ignored",
			auth:      `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct"}}`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
		},
		{
			name:      "malformed config.toml silently ignored",
			auth:      `{"tokens":{"access_token":"canary-secret-token-123","account_id":"acct"}}`,
			config:    `{bad`,
			wantToken: "canary-secret-token-123",
			wantAcct:  "acct",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			authPath := filepath.Join(dir, "auth.json")
			configPath := filepath.Join(dir, "config.toml")
			if tc.auth != "" {
				writeFile(t, authPath, tc.auth)
			}
			if tc.config != "" {
				writeFile(t, configPath, tc.config)
			}
			cred, err := LoadCredential(authPath, configPath)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("LoadCredential() = %+v, want error %q", cred, tc.wantErr)
				}
				if got := err.Error(); got != tc.wantErr {
					t.Errorf("error = %q, want %q", got, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadCredential() error: %v", err)
			}
			if cred.Token != tc.wantToken {
				t.Errorf("Token = %q, want %q", cred.Token, tc.wantToken)
			}
			if cred.AccountID != tc.wantAcct {
				t.Errorf("AccountID = %q, want %q", cred.AccountID, tc.wantAcct)
			}
			if cred.ConfiguredBaseURL != tc.wantBase {
				t.Errorf("ConfiguredBaseURL = %q, want %q", cred.ConfiguredBaseURL, tc.wantBase)
			}
		})
	}
}

// TestLoadCredentialFallthrough pins the non-object nested value rule: a
// non-object "tokens" falls through to "auth", then to the flat value.
func TestLoadCredentialFallthrough(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	configPath := filepath.Join(dir, "config.toml")
	writeFile(t, authPath, `{"tokens":"not-an-object","auth":{"access_token":"canary-secret-token-123","account_id":"acct"}}`)
	cred, err := LoadCredential(authPath, configPath)
	if err != nil {
		t.Fatalf("LoadCredential() error: %v", err)
	}
	if cred.Token != "canary-secret-token-123" || cred.AccountID != "acct" {
		t.Errorf("cred = %+v, want token canary-secret-token-123 + acct", cred)
	}
}
