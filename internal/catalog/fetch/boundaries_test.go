package fetch_test

import (
	"context"
	"crypto/tls"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/aa"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// globalFailureCodes is the global Failure.Code vocabulary
// (specs/global/CONTRACTS.md §1.6) plus the F08-owned missing_api_key.
var globalFailureCodes = map[string]bool{
	"access_denied":            true,
	"config_missing":           true,
	"config_invalid":           true,
	"credential_file":          true,
	"credential_invalid":       true,
	"directory_missing":        true,
	"endpoint_refused":         true,
	"file_unreadable":          true,
	"missing_api_key":          true, // F08-owned
	"network":                  true,
	"provider_status":          true,
	"rate_limited":             true,
	"redirect_refused":         true,
	"response_json":            true,
	"response_too_large":       true,
	"timeout":                  true,
	"unauthorized":             true,
	"unknown":                  true,
	"unreachable":              true,
	"unsupported_response":     true,
	"untrusted_origin":         true,
	"unsupported_config":       true,
	"upstream_unsupported":     true,
	"output_unwritable":        true,
	"io_error":                 true,
	"context_cancelled":        true,
	"schema_validation_failed": true,
	"invalid_arguments":        true,
	"internal":                 true,
}

var forbiddenImports = []string{
	"github.com/WD-Mitchell/which-model/internal/config",
	"github.com/WD-Mitchell/which-model/internal/usage",
	"github.com/WD-Mitchell/which-model/internal/routing",
	"github.com/WD-Mitchell/which-model/internal/pick",
	"github.com/WD-Mitchell/which-model/internal/catalog/csvstore",
	"github.com/WD-Mitchell/which-model/internal/catalog/score",
}

func TestFetchImportBoundaries(t *testing.T) {
	// Repo root is three levels up from this package's directory.
	fetchDir := filepath.Join("..", "..", "..", "internal", "catalog", "fetch")
	packageImports := map[string]map[string]bool{}

	err := filepath.WalkDir(fetchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		pkg := f.Name.Name
		if packageImports[pkg] == nil {
			packageImports[pkg] = map[string]bool{}
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			packageImports[pkg][p] = true
			for _, bad := range forbiddenImports {
				if p == bad {
					t.Errorf("%s imports forbidden package %s", path, p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", fetchDir, err)
	}

	for _, pkg := range []string{"modelsdev", "aa"} {
		imps := packageImports[pkg]
		for _, want := range []string{
			"github.com/WD-Mitchell/which-model/internal/httpkit",
			"github.com/WD-Mitchell/which-model/internal/catalog/identity",
			"github.com/WD-Mitchell/which-model/internal/decimal",
		} {
			if !imps[want] {
				t.Errorf("package %s does not import %s", pkg, want)
			}
		}
	}
}

// installBoundaryTransport lets aa.FetchAAv2 (which pins the real PrimaryURL)
// reach a local TLS test server: https stays intact for the allow-list check,
// but the dial is redirected and the test CA is trusted.
func installBoundaryTransport(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if host == "artificialanalysis.ai" {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	t.Cleanup(func() { http.DefaultTransport = old })
}

// boundaryStatusCode runs aa.FetchAAv2 (the constants wrapper) against a
// server answering every path with the given status and returns the mapped
// fetch.Error code.
func boundaryStatusCode(t *testing.T, status int) string {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status == http.StatusFound {
			w.Header().Set("Location", "https://evil.example/steal")
		}
		w.WriteHeader(status)
	}))
	installBoundaryTransport(t, srv)
	defer srv.Close()

	_, err := aa.FetchAAv2(httpkit.NewClient(), "boundary-test-key")
	var fe *fetch.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fetch.Error", err)
	}
	return fe.Code
}

func TestFetchErrorCodeMapping(t *testing.T) {
	tests := []struct {
		name    string
		produce func() string
	}{
		{
			name: "missing key -> missing_api_key",
			produce: func() string {
				var fe *fetch.Error
				if !errors.As(fetch.MissingAPIKeyError(), &fe) {
					t.Fatal("MissingAPIKeyError is not a *fetch.Error")
				}
				return fe.Code
			},
		},
		{"http 401 -> unauthorized", func() string { return boundaryStatusCode(t, http.StatusUnauthorized) }},
		{"http 429 -> rate_limited", func() string { return boundaryStatusCode(t, http.StatusTooManyRequests) }},
		{"http 500 -> provider_status", func() string { return boundaryStatusCode(t, http.StatusInternalServerError) }},
		{"http 302 -> redirect_refused", func() string { return boundaryStatusCode(t, http.StatusFound) }},
	}
	produced := map[string]bool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := tt.produce()
			produced[code] = true
			if !globalFailureCodes[code] {
				t.Errorf("code %q is not a member of the global Failure.Code set", code)
			}
		})
	}
	// Every produced code must be a member of the global §1.6 set plus
	// missing_api_key (asserted per-case above); also pin the exact mapping
	// of the five scenarios.
	if !produced["missing_api_key"] || !produced["unauthorized"] || !produced["rate_limited"] ||
		!produced["provider_status"] || !produced["redirect_refused"] {
		t.Errorf("produced codes = %v, want all five mapped codes", produced)
	}
}

func TestMissingAPIKeyCodeAndExitNote(t *testing.T) {
	err := fetch.MissingAPIKeyError()
	if err.Code != "missing_api_key" {
		t.Errorf("Code = %q, want missing_api_key (F23 maps to exit 2)", err.Code)
	}
	want := "missing ARTIFICIAL_ANALYSIS_API environment variable or .env entry"
	if msg := err.Error(); msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}
