// Package advisory describes quota evidence without authorizing or blocking work.
package advisory

import (
	"time"

	"github.com/WD-Mitchell/which-model/internal/pick/band"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

const (
	Current             = "current"
	Missing             = "missing"
	Unknown             = "unknown"
	Partial             = "partial"
	Stale               = "stale"
	AuthenticationError = "authentication_error"
	ProviderError       = "provider_error"
	Disabled            = "disabled"
)

type Report struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

func ForState(state string) Report {
	message := "Quota evidence is unavailable; allowance is unconfirmed."
	switch state {
	case Current:
		message = "Quota evidence is current; future allowance is not guaranteed."
	case Missing:
		message = "Quota evidence is missing; allowance is unconfirmed."
	case Partial:
		message = "Quota evidence is partial; allowance is unconfirmed."
	case Stale:
		message = "Quota evidence is stale; allowance is unconfirmed."
	case AuthenticationError:
		message = "Usage authentication evidence is unavailable; allowance is unconfirmed."
	case ProviderError:
		message = "Provider evidence is unavailable; allowance is unconfirmed."
	case Disabled:
		message = "Score-only recommendation; quota was not evaluated."
	case Unknown:
	default:
		state = Unknown
	}
	return Report{State: state, Message: message}
}

// Evaluate uses only this route's observation. It does not substitute a different
// provider's state, refresh data, persist identity or decide whether to launch.
func Evaluate(enabled bool, snapshot *usage.Snapshot, windowIDs []string, maxAge time.Duration, now time.Time) Report {
	if !enabled {
		return ForState(Disabled)
	}
	if snapshot == nil {
		return ForState(Missing)
	}
	if snapshot.Failure != nil {
		switch snapshot.Failure.Code {
		case "unauthorized", "login_required", "expired_credential", "credential_file", "credential_json", "unsafe_credential", "access_denied", "device_expired", "cookie_unavailable", "signing_failed", "keychain_unavailable":
			return ForState(AuthenticationError)
		default:
			return ForState(ProviderError)
		}
	}
	if snapshot.Stale || snapshot.FetchedAt.IsZero() || snapshot.FetchedAt.After(now) || (maxAge > 0 && now.Sub(snapshot.FetchedAt) > maxAge) {
		return ForState(Stale)
	}
	return ForState(windowState(snapshot, windowIDs))
}

// HasCompleteWindows reports whether all of this route's required windows have
// computable readings, independently of their age. Historical numeric evidence
// needs this check even when Evaluate reports Stale before checking coverage.
func HasCompleteWindows(snapshot *usage.Snapshot, windowIDs []string) bool {
	return windowState(snapshot, windowIDs) == Current
}

func windowState(snapshot *usage.Snapshot, windowIDs []string) string {
	if snapshot == nil || snapshot.Failure != nil || !snapshot.UsageKnown || len(windowIDs) == 0 {
		return Unknown
	}
	seen := map[string]bool{}
	known, required := 0, 0
	for _, id := range windowIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		required++
		for _, window := range snapshot.Windows {
			if window.ID == id {
				if _, ok := band.WindowPercent(window); ok {
					known++
				}
				break
			}
		}
	}
	if known == 0 {
		return Unknown
	}
	if known < required {
		return Partial
	}
	return Current
}
