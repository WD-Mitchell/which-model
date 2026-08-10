//go:build !nousage

package usage

import (
	"errors"
	"fmt"
)

// AuthKind discriminates an AuthSource's populated fields / a Credential's origin.
// Exactly one kind-tagged sub-spec on an AuthSource is non-nil for a given Kind value.
type AuthKind int

const (
	AuthEnvVar AuthKind = iota
	AuthFile
	AuthKeychainGeneric
	AuthKeychainInternet
	AuthBrowserCookie
	AuthCLIShellOut
	AuthSubprocessRPC
	AuthOAuthDeviceFlow
	AuthOAuthRefreshGrant
	AuthAWSSigV4
	AuthVolcengineAKSK
	AuthGRPCWebToken // Connect-RPC / gRPC-Web / raw-protobuf token carriers
)

// String renders the AuthKind for humans.
func (k AuthKind) String() string {
	switch k {
	case AuthEnvVar:
		return "env"
	case AuthFile:
		return "file"
	case AuthKeychainGeneric:
		return "keychain-generic"
	case AuthKeychainInternet:
		return "keychain-internet"
	case AuthBrowserCookie:
		return "cookie"
	case AuthCLIShellOut:
		return "cli"
	case AuthSubprocessRPC:
		return "rpc"
	case AuthOAuthDeviceFlow:
		return "oauth-device"
	case AuthOAuthRefreshGrant:
		return "oauth-refresh"
	case AuthAWSSigV4:
		return "aws-sigv4"
	case AuthVolcengineAKSK:
		return "volcengine-aksk"
	case AuthGRPCWebToken:
		return "grpc-web-token"
	default:
		return "unknown"
	}
}

// Credential is the resolved secret handed to a FetchFunc. It never
// round-trips through logs, errors, or Failure.Message (global SPEC §6 invariant 5).
type Credential struct {
	Token  string            // opaque bearer/API token, already ValidateOpaqueToken-validated
	Extra  map[string]string // secondary fields: account_id, project_id, cookie header, ...
	Source AuthKind          // which AuthSource kind in the chain produced it
	Mode   uint32            // credential file POSIX mode, when Source == AuthFile (0 otherwise)
}

// String returns a redacted rendering; NEVER contains Token or any Extra value.
// e.g. `Credential{source=file, token=<redacted>}`.
func (c Credential) String() string {
	return fmt.Sprintf("Credential{source=%s, token=<redacted>}", c.Source.String())
}

// FailureError carries a stable Failure.Code through the error path.
// Every resolver, the fetch layer, and providers construct/consume this type.
type FailureError struct {
	Failure Failure
}

func (e *FailureError) Error() string {
	return e.Failure.Code + ": " + e.Failure.Message
}

// NewFailureError builds an error carrying the given canonical Failure.Code.
// message MUST be sanitised (no credential material) at the call site.
func NewFailureError(code, message string) error {
	return &FailureError{Failure: Failure{Code: code, Message: message}}
}

// AsFailure extracts a Failure from err; ok=false when err is not (or does not
// wrap) a *FailureError.
func AsFailure(err error) (Failure, bool) {
	var fe *FailureError
	if errors.As(err, &fe) {
		return fe.Failure, true
	}
	return Failure{}, false
}
