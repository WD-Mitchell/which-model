package aa

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// writeEnvFile writes a .env with the given lines into a temp dir and
// returns the dir path (repoRoot) plus a cleanup.
func writeEnvFile(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return dir
}

func TestLoadAAAPIKeyEnvWins(t *testing.T) {
	dir := writeEnvFile(t, `ARTIFICIAL_ANALYSIS_API="file-key-456"`)
	t.Setenv(APIKeyEnv, "env-key-123")
	got, err := LoadAAAPIKey(dir)
	if err != nil {
		t.Fatalf("LoadAAAPIKey: %v", err)
	}
	if got != "env-key-123" {
		t.Errorf("got %q, want env-key-123 (env wins over .env)", got)
	}
}

func TestLoadAAAPIKeyEnvBlank(t *testing.T) {
	dir := writeEnvFile(t, `ARTIFICIAL_ANALYSIS_API="file-key-456"`)
	t.Setenv(APIKeyEnv, "   ")
	got, err := LoadAAAPIKey(dir)
	if err != nil {
		t.Fatalf("LoadAAAPIKey: %v", err)
	}
	if got != "file-key-456" {
		t.Errorf("got %q, want file-key-456 (blank env falls back to .env)", got)
	}
}

func TestLoadAAAPIKeyFromEnvFile(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "double quotes stripped",
			lines: []string{`ARTIFICIAL_ANALYSIS_API="file-key-456"`},
			want:  "file-key-456",
		},
		{
			name:  "single quotes stripped",
			lines: []string{`ARTIFICIAL_ANALYSIS_API='file-key-456'`},
			want:  "file-key-456",
		},
		{
			name:  "unquoted verbatim",
			lines: []string{`ARTIFICIAL_ANALYSIS_API=file-key-456`},
			want:  "file-key-456",
		},
		{
			name:  "surrounding noise skipped",
			lines: []string{
				"",
				"# a comment line",
				"OTHER=ignored",
				`ARTIFICIAL_ANALYSIS_API="file-key-456"`,
			},
			want: "file-key-456",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeEnvFile(t, tt.lines...)
			got, err := LoadAAAPIKey(dir)
			if err != nil {
				t.Fatalf("LoadAAAPIKey: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadAAAPIKeyMissing(t *testing.T) {
	// .env present but without the key.
	dir := writeEnvFile(t, "OTHER=value")
	_, err := LoadAAAPIKey(dir)
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "missing_api_key" {
		t.Errorf("Code = %q, want missing_api_key", fe.Code)
	}
	want := "missing ARTIFICIAL_ANALYSIS_API environment variable or .env entry"
	if msg := fe.Error(); msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}

	// No .env at all, no env.
	t.Setenv(APIKeyEnv, "")
	_, err = LoadAAAPIKey(t.TempDir())
	var fe2 *fetch.Error
	if !errors.As(err, &fe2) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe2.Code != "missing_api_key" {
		t.Errorf("Code = %q, want missing_api_key", fe2.Code)
	}
	if msg := fe2.Error(); msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestLoadAAAPIKeyMissingEnvFile(t *testing.T) {
	// .env file missing entirely (and no env var): MissingAPIKeyError, not
	// credential_file — the code documents both missing sources together.
	t.Setenv(APIKeyEnv, "")
	_, err := LoadAAAPIKey(t.TempDir())
	if err == nil {
		t.Fatal("LoadAAAPIKey: nil error, want missing_api_key")
	}
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "missing_api_key" {
		t.Errorf("Code = %q, want missing_api_key", fe.Code)
	}
}

func TestLoadAAAPIKeyEnvFileErrors(t *testing.T) {
	const key = "super-secret-aa-key"

	t.Run("duplicate key", func(t *testing.T) {
		dir := writeEnvFile(t,
			"ARTIFICIAL_ANALYSIS_API="+key,
			"ARTIFICIAL_ANALYSIS_API="+key,
		)
		_, err := LoadAAAPIKey(dir)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "credential_file" {
			t.Errorf("Code = %q, want credential_file", fe.Code)
		}
		if msg := fe.Error(); strings.Contains(msg, key) {
			t.Errorf("error %q leaks the key", msg)
		}
	})

	t.Run("malformed line", func(t *testing.T) {
		dir := writeEnvFile(t,
			"ARTIFICIAL_ANALYSIS_API="+key,
			"noequals-this-is-not-a-line",
		)
		_, err := LoadAAAPIKey(dir)
		var fe *fetch.Error
		if !errors.As(err, &fe) {
			t.Fatalf("error = %v, want *fetch.Error", err)
		}
		if fe.Code != "credential_file" {
			t.Errorf("Code = %q, want credential_file", fe.Code)
		}
		want := "invalid .env line at " + filepath.Join(dir, ".env") + ":2"
		if msg := fe.Error(); msg != want {
			t.Errorf("message = %q, want %q", msg, want)
		}
		if strings.Contains(fe.Error(), key) {
			t.Errorf("error %q leaks the key", fe.Error())
		}
	})
}

func TestLoadAAAPIKeyIgnoresHistoricalAlias(t *testing.T) {
	// AA_API_KEY is a historical alias of the env var; the .env reader only
	// honors ARTIFICIAL_ANALYSIS_API (annex-b §2.3).
	dir := writeEnvFile(t, "AA_API_KEY=old-name-key")
	t.Setenv(APIKeyEnv, "")
	_, err := LoadAAAPIKey(dir)
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	if fe.Code != "missing_api_key" {
		t.Errorf("Code = %q, want missing_api_key", fe.Code)
	}
}

func TestAAV2ClientTimeout(t *testing.T) {
	c := AAV2Client()
	timeout := reflect.ValueOf(c).Elem().FieldByName("timeout").Int()
	if timeout != int64(20*time.Second) {
		t.Errorf("timeout = %v, want 20s", time.Duration(timeout))
	}
}

func TestAAPageClientLimits(t *testing.T) {
	c := AAPageClient()
	v := reflect.ValueOf(c).Elem()
	if got := v.FieldByName("timeout").Int(); got != int64(20*time.Second) {
		t.Errorf("timeout = %v, want 20s", time.Duration(got))
	}
	if got := v.FieldByName("maxBytes").Int(); got != int64(2<<20) {
		t.Errorf("maxBytes = %v, want 2 MiB", got)
	}
}

func TestAAPageClientIsHTTPKitClient(t *testing.T) {
	// The client constructors must return the httpkit client type so the
	// collectors' SetAllowList/Do contract (F04) applies.
	if c := AAV2Client(); c == nil {
		t.Fatal("AAV2Client() = nil")
	} else if _, ok := any(c).(*httpkit.Client); !ok {
		t.Fatalf("AAV2Client() = %T, want *httpkit.Client", c)
	}
}
