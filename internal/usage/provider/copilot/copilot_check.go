//go:build !nousage

package copilot

import (
	"context"
	"net/http"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// Fetch is the FetchFunc port of checkCopilotUsage (copilot.mjs:244-285)
// minus the interactive --login leg (SPEC §2.3-§2.4, §2.10): empty cred.Token
// → Failure{Code: "login_required", Message: "No usable GitHub token was
// found; rerun with --login to start device login."} with no HTTP calls; else
// ValidateIdentity (a failure is a hard Failure — never a fallback to other
// sources), then fetchUsage; Snapshot.Account = login. The usage call MUST
// NOT happen unless ValidateIdentity succeeded in this same run (SPEC D3).
func Fetch(ctx context.Context, cred usage.Credential, client *http.Client) (usage.Snapshot, error) {
	if cred.Token == "" {
		return usage.Snapshot{
			Provider: "copilot",
			Failure: &usage.Failure{
				Code:    "login_required",
				Message: "No usable GitHub token was found; rerun with --login to start device login.",
			},
			FetchedAt:  time.Now().UTC(),
			Source:     usage.SourceOAuth,
			Confidence: "live",
		}, nil
	}

	login, err := ValidateIdentity(ctx, cred.Token, client)
	if err != nil {
		return failureSnapshot(err), nil
	}

	windows, err := fetchUsage(ctx, cred.Token, client)
	if err != nil {
		return failureSnapshot(err), nil
	}

	return usage.Snapshot{
		Provider:   "copilot",
		Windows:    windows,
		Account:    login,
		FetchedAt:  time.Now().UTC(),
		Source:     usage.SourceOAuth,
		Confidence: "live",
	}, nil
}

// failureSnapshot assembles a Snapshot carrying the provider error as its
// Failure (CONTRACTS §8.5: provider failures ride in the Snapshot; the error
// return is reserved for programming errors). The fallback branch is
// unreachable in practice — every error from this package is a *Error — and
// uses a fixed sanitized string so no underlying error text can leak.
func failureSnapshot(err error) usage.Snapshot {
	s := usage.Snapshot{
		Provider:   "copilot",
		FetchedAt:  time.Now().UTC(),
		Source:     usage.SourceOAuth,
		Confidence: "live",
	}
	if pe, ok := err.(*Error); ok {
		s.Failure = &usage.Failure{Code: pe.Code, Message: pe.Message}
	} else {
		s.Failure = &usage.Failure{Code: "provider_status", Message: "GitHub Copilot usage is unavailable."}
	}
	return s
}
