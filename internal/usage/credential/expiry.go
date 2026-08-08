//go:build !nousage

package credential

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// epochSeconds converts a numeric epoch to a time.Time using the
// seconds-vs-milliseconds heuristic: n > 10_000_000_000 is milliseconds
// (prototype resetText, usage-allowance-checks-spec.md §1).
func epochSeconds(n float64) (time.Time, error) {
	if n > 10_000_000_000 {
		return time.UnixMilli(int64(n)), nil
	}
	return time.Unix(int64(n), 0), nil
}

// ParseExpiry converts a decoded JSON value to a time.Time: number epochs
// with the >10_000_000_000 = milliseconds heuristic (prototype resetText);
// strings parsed as RFC3339/RFC3339Nano or as numeric strings.
func ParseExpiry(v any) (time.Time, error) {
	switch n := v.(type) {
	case float64:
		return epochSeconds(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return time.Time{}, err
		}
		return epochSeconds(f)
	case int:
		return epochSeconds(float64(n))
	case int64:
		return epochSeconds(float64(n))
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return time.Time{}, fmt.Errorf("credential expiry is empty")
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return epochSeconds(f)
		}
		return time.Time{}, fmt.Errorf("credential expiry is not a valid epoch or timestamp")
	default:
		return time.Time{}, fmt.Errorf("credential expiry is not a valid epoch or timestamp")
	}
}

// CheckExpired returns nil when exp is after now, else an
// expired_credential *usage.FailureError.
func CheckExpired(exp time.Time, now time.Time) error {
	if now.After(exp) {
		return usage.NewFailureError("expired_credential", "credential expired")
	}
	return nil
}
