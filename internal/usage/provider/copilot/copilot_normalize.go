//go:build !nousage

package copilot

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// windowKeys is the fixed .mjs iteration order (copilot.mjs:197-223), not map
// order: chat, completions, premium_interactions.
var windowKeys = []string{"chat", "completions", "premium_interactions"}

// NormalizeUsage is the port of normalizeCopilotUsage (copilot.mjs:197-223)
// per SPEC §2.8. quota_snapshots must be a non-array object; windows are
// produced in the fixed order chat, completions, premium_interactions with
// IDs premium/chat/completions, labels = key with "_" → space, Unit requests,
// UsageKnown true. Per window: Unlimited = (unlimited === true);
// Remaining = remaining (finite ≥ 0); Limit = entitlement;
// UsedPercent = 100 - percent_remaining when present (SPEC D7);
// ResetsAt = resetTime(reset_at ?? quota_reset_date); skipped unless
// unlimited OR remaining OR percent_remaining present (entitlement alone is
// not enough). Zero windows → Error{Code: "unsupported_response",
// Message: "GitHub Copilot returned an unsupported usage shape."}.
func NormalizeUsage(raw []byte) ([]usage.Window, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}
	snapshotsRaw, ok := value["quota_snapshots"]
	if !ok || !isObject(snapshotsRaw) {
		return nil, &Error{Code: "unsupported_response", Message: "GitHub Copilot returned an unsupported usage shape."}
	}
	var snapshots map[string]json.RawMessage
	if err := json.Unmarshal(snapshotsRaw, &snapshots); err != nil {
		return nil, &Error{Code: "unsupported_response", Message: "GitHub Copilot returned an unsupported usage shape."}
	}

	windows := make([]usage.Window, 0, len(windowKeys))
	for _, key := range windowKeys {
		srcRaw, ok := snapshots[key]
		if !ok || !isObject(srcRaw) {
			continue
		}
		var src map[string]json.RawMessage
		if err := json.Unmarshal(srcRaw, &src); err != nil {
			continue
		}

		// unlimited := source.unlimited === true (strict boolean).
		unlimited := false
		if b, ok := src["unlimited"]; ok {
			var v bool
			if json.Unmarshal(b, &v) == nil {
				unlimited = v
			}
		}
		remaining := finiteNonNegative(src["remaining"])
		entitlement := finiteNonNegative(src["entitlement"])
		percent := finitePercent(src["percent_remaining"])
		if !unlimited && remaining == nil && percent == nil {
			continue
		}

		w := usage.Window{
			ID:         key,
			Label:      strings.ReplaceAll(key, "_", " "),
			Unit:       usage.UnitRequests,
			Unlimited:  unlimited,
			UsageKnown: true,
		}
		if key == "premium_interactions" {
			w.ID = "premium"
		}
		if remaining != nil {
			w.Remaining = remaining
		}
		if entitlement != nil {
			w.Limit = entitlement
		}
		if percent != nil {
			used := 100 - *percent
			w.UsedPercent = &used
		}
		if resets := resetTime(resetRaw(src, value)); resets != nil {
			w.ResetsAt = resets
		}
		windows = append(windows, w)
	}

	if len(windows) == 0 {
		return nil, &Error{Code: "unsupported_response", Message: "GitHub Copilot returned an unsupported usage shape."}
	}
	return windows, nil
}

// resetRaw implements source.reset_at ?? quota_reset_date (nullish
// coalescing: a present non-null per-window reset_at wins).
func resetRaw(src, top map[string]json.RawMessage) json.RawMessage {
	if r, ok := src["reset_at"]; ok && !isNull(r) {
		return r
	}
	if r, ok := top["quota_reset_date"]; ok && !isNull(r) {
		return r
	}
	return nil
}

// isObject reports whether raw is a JSON object (first non-space byte '{').
func isObject(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) > 0 && t[0] == '{'
}

// isNull reports whether raw is the JSON literal null.
func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// finiteNumber parses a JSON number or numeric string into a finite float64
// (ports of core.mjs:202-224 finiteNumber semantics).
func finiteNumber(raw json.RawMessage) (float64, bool) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return 0, false
	}
	var f float64
	switch t[0] {
	case '"':
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		f = parsed
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if err := json.Unmarshal(t, &f); err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// finiteNonNegative returns a finite number >= 0 (nil otherwise).
func finiteNonNegative(raw json.RawMessage) *float64 {
	n, ok := finiteNumber(raw)
	if !ok || n < 0 {
		return nil
	}
	return &n
}

// finitePercent returns a finite number in [0, 100] (nil otherwise).
func finitePercent(raw json.RawMessage) *float64 {
	n, ok := finiteNumber(raw)
	if !ok || n < 0 || n > 100 {
		return nil
	}
	return &n
}

// resetTime ports resetText (core.mjs / copilot.mjs): an ISO string (date-only
// strings parse at UTC midnight, SPEC D8) or an epoch number (> 10_000_000_000
// is milliseconds, else seconds); unparseable → nil.
func resetTime(raw json.RawMessage) *time.Time {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 {
		return nil
	}
	var tm time.Time
	switch t[0] {
	case '"':
		var s string
		if err := json.Unmarshal(t, &s); err != nil {
			return nil
		}
		for _, layout := range []string{time.RFC3339Nano, time.DateOnly} {
			if parsed, err := time.Parse(layout, s); err == nil {
				tm = parsed
				break
			}
		}
		if tm.IsZero() {
			return nil
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		f, ok := finiteNumber(t)
		if !ok {
			return nil
		}
		if f > 10_000_000_000 {
			tm = time.UnixMilli(int64(f))
		} else {
			tm = time.Unix(int64(f), 0)
		}
	default:
		return nil
	}
	return &tm
}
