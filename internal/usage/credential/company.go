//go:build !nousage

package credential

import (
	"os"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

var readCompanyPolicy = company.Load
var managedFileRead = os.ReadFile
var managedFileStat = os.Stat
var managedFileWrite = config.AtomicWriteFile

func sourceCategory(kind usage.AuthKind) string {
	switch kind {
	case usage.AuthKeychainGeneric, usage.AuthKeychainInternet:
		return "keychain"
	case usage.AuthEnvVar:
		return "environment"
	case usage.AuthFile, usage.AuthBrowserCookie:
		return "provider_file"
	case usage.AuthCLIShellOut, usage.AuthSubprocessRPC:
		return "cli"
	default:
		return "unsupported"
	}
}

func requireSource(source string) error {
	policy, err := readCompanyPolicy()
	if err != nil {
		return err
	}
	// A legacy PATH-based credential helper is not an approved executable form.
	if policy.Managed && source == "cli" {
		return &company.Error{Origin: policy.Origin, Reason: "credential CLI execution requires an approved executable implementation"}
	}
	return policy.RequireSource(source)
}
