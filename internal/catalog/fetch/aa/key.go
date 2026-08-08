package aa

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// APIKeyEnv is the canonical environment variable holding the Artificial
// Analysis API key. AA_API_KEY is recorded as a historical alias of this
// env var; the loader honors only APIKeyEnv (annex-b §2.3).
const APIKeyEnv = "ARTIFICIAL_ANALYSIS_API"

// LoadAAAPIKey resolves the AA API key: the environment variable wins;
// otherwise the repo-root .env is scanned for an ARTIFICIAL_ANALYSIS_API
// entry (blank lines and # comments skipped, one layer of matching quotes
// stripped, other keys ignored). Neither source present -> MissingAPIKeyError.
// .env problems (unreadable file, malformed line, duplicate entry) are
// credential_file errors whose text never echoes the key value.
func LoadAAAPIKey(repoRoot string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(APIKeyEnv)); v != "" {
		return v, nil
	}

	path := filepath.Join(repoRoot, ".env")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fetch.MissingAPIKeyError()
		}
		return "", &fetch.Error{
			Code: "credential_file",
			Err:  fmt.Errorf("cannot read %s", path),
		}
	}

	var key string
	found := false
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if !ok {
			return "", &fetch.Error{
				Code: "credential_file",
				Err:  fmt.Errorf("invalid .env line at %s:%d", path, i+1),
			}
		}
		if strings.TrimSpace(k) != APIKeyEnv {
			continue // other keys are ignored (forward compatibility)
		}
		v = strings.TrimSpace(v)
		v = stripOneLayerOfQuotes(v)
		if v == "" {
			continue // blank value counts as absent
		}
		if found {
			return "", &fetch.Error{
				Code: "credential_file",
				Err:  fmt.Errorf("duplicate %s entry in %s", APIKeyEnv, path),
			}
		}
		key = v
		found = true
	}
	if !found {
		return "", fetch.MissingAPIKeyError()
	}
	return key, nil
}

// stripOneLayerOfQuotes removes one layer of matching surrounding single or
// double quotes, exactly as a shell would read the value.
func stripOneLayerOfQuotes(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// AAV2Client is the client for the AA v2 API collector family: 20s timeout,
// default response-size cap.
func AAV2Client() *httpkit.Client {
	return httpkit.NewClient(httpkit.WithTimeout(20 * time.Second))
}

// AAPageClient is the client for the AA model-page scraper family: 20s
// timeout and a 2 MiB response cap (pages are bounded HTML/JS payloads).
func AAPageClient() *httpkit.Client {
	return httpkit.NewClient(httpkit.WithTimeout(20*time.Second), httpkit.WithMaxBytes(2<<20))
}
