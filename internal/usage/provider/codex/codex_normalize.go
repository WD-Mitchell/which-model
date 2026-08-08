//go:build !nousage

package codex

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// finitePercent ports finitePercent (core.mjs:202-205): accepts a JSON number
// or a numeric string; returns nil unless the value is a finite number in
// [0, 100].
func finitePercent(raw json.RawMessage) *float64 {
	f, ok := finiteNumber(raw)
	if !ok || f < 0 || f > 100 {
		return nil
	}
	return &f
}

// finiteNonNegative ports finiteNonNegative (core.mjs:207-210): accepts a JSON
// number or a numeric string; returns nil unless the value is a finite number
// >= 0.
func finiteNonNegative(raw json.RawMessage) *float64 {
	f, ok := finiteNumber(raw)
	if !ok || f < 0 {
		return nil
	}
	return &f
}

// finiteNumber decodes a JSON number or numeric string into a finite float.
func finiteNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, false
	}
	var f float64
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, false
		}
		if strings.TrimSpace(s) == "" {
			return 0, false
		}
		var err error
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
	} else if err := json.Unmarshal(raw, &f); err != nil {
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// resetTime ports resetText (core.mjs:212-224): epoch numbers (> 10^10 are
// milliseconds, else seconds) and ISO strings; nil for anything invalid.
func resetTime(raw json.RawMessage) *time.Time {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || len(s) > 128 {
			return nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if ts, err := time.Parse(layout, s); err == nil {
				return &ts
			}
		}
		return nil
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil || !(n > 0) || math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	ms := n
	if n <= 10_000_000_000 {
		ms = n * 1000
	}
	ts := time.UnixMilli(int64(ms)).UTC()
	if ts.Year() < 1 {
		return nil
	}
	return &ts
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug lowercases s and collapses runs of non-alphanumerics into "-",
// trimming leading/trailing dashes.
func slug(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// pickRaw returns the first of keys that is present and not JSON null,
// mirroring the .mjs `??` chain (null/undefined fall through).
func pickRaw(m map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, k := range keys {
		if raw, ok := m[k]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return raw, true
		}
	}
	return nil, false
}

// windowMinutes converts limit_window_seconds to minutes (annex-a §3.1:
// value/60) when the value is a finite positive integer; else nil. The field
// is read from the parent rate-limit object first, then from the window
// source (F16-T4 cases 6-7 place it at the rate-limit level).
func windowMinutes(parent, src map[string]json.RawMessage) *int {
	raw, ok := pickRaw(parent, "limit_window_seconds", "limitWindowSeconds")
	if !ok {
		raw, ok = pickRaw(src, "limit_window_seconds", "limitWindowSeconds")
	}
	if !ok {
		return nil
	}
	f, ok := finiteNumber(raw)
	if !ok || f <= 0 || math.Trunc(f) != f {
		return nil
	}
	m := int(f / 60)
	return &m
}

// windowFromSource builds one percent window from a rate-limit window source
// object. Returns nil when no valid used_percent is present.
func windowFromSource(src map[string]json.RawMessage, parent map[string]json.RawMessage, id, label string) *usage.Window {
	usedRaw, ok := pickRaw(src, "used_percent", "usedPercent")
	if !ok {
		return nil
	}
	used := finitePercent(usedRaw)
	if used == nil {
		return nil
	}
	w := usage.Window{
		ID:          id,
		Label:       label,
		Unit:        usage.UnitPercent,
		UsedPercent: used,
		UsageKnown:  true,
	}
	if resetRaw, ok := pickRaw(src, "reset_at", "resetAt"); ok {
		w.ResetsAt = resetTime(resetRaw)
	}
	w.WindowMinutes = windowMinutes(parent, src)
	return &w
}

// NormalizeUsage is the port of normalizeCodexUsage (codex.mjs:63-81) plus
// the annex-a §3.1 additional_rate_limits mapping (SPEC §2.10-11). Input is
// the raw JSON object body of a 200 response. Window order: 5h, weekly,
// credits, then additional:* in response order. Returns Error{Code:
// "unsupported_response", Message: "Codex returned an unsupported usage
// shape."} when zero windows are produced.
func NormalizeUsage(raw []byte) ([]usage.Window, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}

	rateLimit := value
	if obj, ok := asObjectField(value, "rate_limit"); ok {
		rateLimit = obj
	} else if obj, ok := asObjectField(value, "rateLimit"); ok {
		rateLimit = obj
	}

	windows := make([]usage.Window, 0, 3)

	for _, win := range []struct {
		key   string
		camel string
		id    string
		label string
	}{
		{"primary_window", "primaryWindow", "5h", "primary window"},
		{"secondary_window", "secondaryWindow", "weekly", "secondary window"},
	} {
		srcRaw, ok := pickRaw(rateLimit, win.key, win.camel)
		if !ok {
			continue
		}
		src, ok := asObjectField(map[string]json.RawMessage{win.key: srcRaw}, win.key)
		if !ok {
			continue
		}
		if w := windowFromSource(src, rateLimit, win.id, win.label); w != nil {
			windows = append(windows, *w)
		}
	}

	if credits, ok := asObjectField(value, "credits"); ok {
		if balRaw, ok := pickRaw(credits, "balance"); ok {
			if balance := finiteNonNegative(balRaw); balance != nil {
				windows = append(windows, usage.Window{
					ID:         "credits",
					Label:      "credits",
					Unit:       usage.UnitCredits,
					Remaining:  balance,
					UsageKnown: true,
				})
			}
		}
	}

	windows = append(windows, additionalWindows(value)...)

	if len(windows) == 0 {
		return nil, &Error{Code: "unsupported_response", Message: "Codex returned an unsupported usage shape."}
	}
	return windows, nil
}

// additionalWindows maps value.additional_rate_limits (annex-a §3.1): one
// percent window per entry, ID "additional:<slug(limitName)>", percent from
// the entry's rate-limit primary else secondary window. Entries with no valid
// percent are skipped.
func additionalWindows(value map[string]json.RawMessage) []usage.Window {
	raw, ok := pickRaw(value, "additional_rate_limits", "additionalRateLimits")
	if !ok {
		return nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	out := make([]usage.Window, 0, len(entries))
	for _, entryRaw := range entries {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &entry); err != nil || entry == nil {
			continue
		}
		limitName := ""
		if lr, ok := pickRaw(entry, "limit_name", "limitName"); ok {
			limitName = stringField(lr)
		}
		if limitName == "" {
			continue
		}
		rlRaw, ok := pickRaw(entry, "rate_limit", "rateLimit")
		if !ok {
			continue
		}
		var rl map[string]json.RawMessage
		if err := json.Unmarshal(rlRaw, &rl); err != nil || rl == nil {
			continue
		}
		srcRaw, ok := pickRaw(rl, "primary_window", "primaryWindow")
		if !ok {
			srcRaw, ok = pickRaw(rl, "secondary_window", "secondaryWindow")
		}
		if !ok {
			continue
		}
		var src map[string]json.RawMessage
		if err := json.Unmarshal(srcRaw, &src); err != nil || src == nil {
			continue
		}
		w := windowFromSource(src, rl, "additional:"+slug(limitName), limitName)
		if w == nil {
			continue
		}
		if mfRaw, ok := pickRaw(entry, "metered_feature", "meteredFeature"); ok {
			if mf := stringField(mfRaw); mf != "" {
				w.ModelScope = []string{mf}
			}
		}
		out = append(out, *w)
	}
	return out
}
