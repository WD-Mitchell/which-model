//go:build !nousage

package codex

import "testing"

// TestParseConfig ports the parseCodexConfig table from F16-T2
// (usage-allowance-checks/lib/codex.mjs:8-35).
func TestParseConfig(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "active provider wins (mjs case 5 fixture)",
			input: "model_provider = \"trusted\"\n[model_providers.trusted]\nbase_url = \"https://trusted.example/v1\"",
			want:  "https://trusted.example/v1",
		},
		{
			name:  "single-quoted value",
			input: "model_provider = \"trusted\"\n[model_providers.trusted]\nbase_url = 'https://trusted.example/v1'",
			want:  "https://trusted.example/v1",
		},
		{
			name:  "provider entry without active selection ignored",
			input: "[model_providers.trusted]\nbase_url = \"https://a.example\"",
			want:  "",
		},
		{
			name:  "root base_url only",
			input: "base_url = \"https://root.example\"",
			want:  "https://root.example",
		},
		{
			name:  "active provider wins over root and other providers",
			input: "model_provider = \"trusted\"\nbase_url = \"https://root.example\"\n[model_providers.trusted]\nbase_url = \"https://a.example\"\n[model_providers.other]\nbase_url = \"https://b.example\"",
			want:  "https://a.example",
		},
		{
			name:  "inactive provider ignored",
			input: "model_provider = \"other\"\n[model_providers.trusted]\nbase_url = \"https://a.example\"",
			want:  "",
		},
		{
			name:  "trailing comment stripped",
			input: "model_provider = \"trusted\"\n[model_providers.trusted]\nbase_url = \"https://a.example\" # comment\n",
			want:  "https://a.example",
		},
		{
			name:  "non-provider section ignored",
			input: "[other_section]\nbase_url = \"https://b.example\"",
			want:  "",
		},
		{
			name:  "root assignment after section still scoped",
			input: "model_provider = \"trusted\"\n[model_providers.trusted]\nbase_url = \"https://a.example\"\nbase_url = \"https://root.example\"",
			want:  "https://a.example",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "comment only",
			input: "# only comment",
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseConfig(tc.input); got != tc.want {
				t.Errorf("ParseConfig() = %q, want %q", got, tc.want)
			}
		})
	}
}
