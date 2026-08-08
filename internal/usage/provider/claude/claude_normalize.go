//go:build !nousage

package claude

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

// windowKey describes one fixed snake_case response key and its canonical
// window identity (SPEC §2.7, CONTRACTS §5).
type windowKey struct {
	key          string
	id           string
	label        string
	minutes      int
	modelScope   []string
}

// fixedWindowKeys are probed in fixed order (SPEC §2.7).
var fixedWindowKeys = []windowKey{
	{key: "five_hour", id: "5h", label: "five hour", minutes: 300},
	{key: "seven_day", id: "weekly", label: "seven day", minutes: 10080},
	{key: "seven_day_sonnet", id: "sonnet_7d", label: "seven day Sonnet", minutes: 10080, modelScope: []string{"sonnet"}},
	{key: "seven_day_opus", id: "opus_7d", label: "seven day Opus", minutes: 10080, modelScope: []string{"opus"}},
	{key: "seven_day_oauth_apps", id: "oauth_apps_7d", label: "seven day OAuth apps", minutes: 10080},
}

// routinesTryKeys are tried in order; the first key present with a non-null
// object wins (annex-a §3.2, survey:136-143).
var routinesTryKeys = []string{
	"seven_day_routines",
	"seven_day_claude_routines",
	"claude_routines",
	"routines",
	"routine",
	"seven_day_cowork",
	"cowork",
}

// NormalizeUsage is the port of normalizeClaudeUsage (claude.mjs:33-56) plus
// the annex-a §3.2 extraUsage/limits mapping (SPEC §2 items 7-9). Input is the
// raw JSON object body of a 200 response. Returns windows in fixed order:
// 5h, weekly, sonnet_7d, opus_7d, oauth_apps_7d, routines_7d, extra_usage,
// then dynamic limit:* windows in response order. Returns Error{Code:
// "unsupported_response", Message: "Claude returned an unsupported usage
// shape."} when zero windows are produced and the 5h synthetic rule does not
// apply.
func NormalizeUsage(raw []byte) ([]usage.Window, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, &Error{Code: "response_json", Message: "The provider returned unsupported JSON."}
	}

	var windows []usage.Window

	// five_hour first, with the D5 synthetic rule: present-but-null or
	// present-but-not-an-object emits a synthetic 5h window; absent emits
	// nothing.
	if five, ok := obj["five_hour"]; ok {
		var fiveObj map[string]json.RawMessage
		if json.Unmarshal(five, &fiveObj) == nil && fiveObj != nil {
			if w := buildFixedWindow(fixedWindowKeys[0], fiveObj); w != nil {
				windows = append(windows, *w)
			}
		} else {
			windows = append(windows, usage.Window{
				ID:         "5h",
				Label:      "five hour",
				Unit:       usage.UnitPercent,
				Synthetic:  true,
				UsageKnown: false,
			})
		}
	}

	for _, wk := range fixedWindowKeys[1:] {
		rawObj, ok := obj[wk.key]
		if !ok {
			continue
		}
		var wObj map[string]json.RawMessage
		if json.Unmarshal(rawObj, &wObj) != nil || wObj == nil {
			continue // absent or not an object → skip
		}
		if w := buildFixedWindow(wk, wObj); w != nil {
			windows = append(windows, *w)
		}
	}

	// routines try-key chain: first present non-null object wins.
	for _, key := range routinesTryKeys {
		rawObj, ok := obj[key]
		if !ok {
			continue
		}
		var rObj map[string]json.RawMessage
		if json.Unmarshal(rawObj, &rObj) != nil || rObj == nil {
			continue
		}
		if w := buildFixedWindow(windowKey{id: "routines_7d", label: "seven day Routines", minutes: 10080}, rObj); w != nil {
			windows = append(windows, *w)
		}
		break
	}

	// extraUsage (SPEC §2.9): one usd window when at least one of
	// usedCredits / monthlyLimit / utilization is valid.
	if extraRaw, ok := obj["extraUsage"]; ok {
		var extra map[string]json.RawMessage
		if json.Unmarshal(extraRaw, &extra) == nil && extra != nil {
			w := buildExtraUsage(extra)
			if w != nil {
				windows = append(windows, *w)
			}
		}
	}

	// limits (SPEC §2.9): dynamic limit:<slug> windows in response order.
	if limitsRaw, ok := obj["limits"]; ok {
		var limits []json.RawMessage
		if json.Unmarshal(limitsRaw, &limits) == nil {
			for _, entry := range limits {
				var lim map[string]json.RawMessage
				if json.Unmarshal(entry, &lim) != nil || lim == nil {
					continue
				}
				if w := buildLimitWindow(lim); w != nil {
					windows = append(windows, *w)
				}
			}
		}
	}

	if len(windows) == 0 {
		return nil, &Error{Code: "unsupported_response", Message: "Claude returned an unsupported usage shape."}
	}
	return windows, nil
}

// buildFixedWindow ports the per-window mapping (claude.mjs:37-44):
// usedPercent = finitePercent(utilization ?? used_percent), resetsAt =
// resetTime(resets_at ?? reset_at). Returns nil when the source is absent or
// its utilization is invalid.
func buildFixedWindow(wk windowKey, src map[string]json.RawMessage) *usage.Window {
	used, ok := pickFinitePercent(src, "utilization", "used_percent")
	if !ok {
		return nil
	}
	min := wk.minutes
	return &usage.Window{
		ID:            wk.id,
		Label:         wk.label,
		Unit:          usage.UnitPercent,
		UsedPercent:   &used,
		WindowMinutes: &min,
		ResetsAt:      pickResetTime(src, "resets_at", "reset_at"),
		ModelScope:    wk.modelScope,
		UsageKnown:    true,
	}
}

// buildExtraUsage maps extraUsage (annex-a §3.2, survey:136-143). At least one
// valid field among usedCredits (finite ≥ 0), monthlyLimit (finite ≥ 0),
// utilization (0..100) is required; isEnabled is not consulted (SPEC D10).
func buildExtraUsage(extra map[string]json.RawMessage) *usage.Window {
	w := usage.Window{
		ID:         "extra_usage",
		Label:      "Extra usage",
		Unit:       usage.UnitUSD,
		UsageKnown: true,
	}
	valid := false
	if used, ok := finiteNonNegative(extra["usedCredits"]); ok {
		w.Used = &used
		valid = true
	}
	if limit, ok := finiteNonNegative(extra["monthlyLimit"]); ok {
		w.Limit = &limit
		valid = true
	}
	if pct, ok := finitePercent(extra["utilization"]); ok {
		w.UsedPercent = &pct
		valid = true
	}
	if !valid {
		return nil
	}
	return &w
}

// buildLimitWindow maps one limits[i] entry (annex-a §3.2). Requires a valid
// percent (0..100); isActive is not consulted (SPEC D10).
func buildLimitWindow(lim map[string]json.RawMessage) *usage.Window {
	pct, ok := finitePercent(lim["percent"])
	if !ok {
		return nil
	}
	kind, _ := stringField(lim, "kind")
	group, _ := stringField(lim, "group")
	label := group
	if label == "" {
		label = kind
	}
	w := usage.Window{
		ID:          "limit:" + slug(kind+"_"+group),
		Label:       label,
		Unit:        usage.UnitPercent,
		UsedPercent: &pct,
		ResetsAt:    resetTime(lim["resetsAt"]),
		UsageKnown:  true,
	}
	if scopeRaw, ok := lim["scope"]; ok {
		var scope struct {
			Model struct {
				ID string `json:"id"`
			} `json:"model"`
		}
		if json.Unmarshal(scopeRaw, &scope) == nil && scope.Model.ID != "" {
			w.ModelScope = []string{scope.Model.ID}
		}
	}
	return &w
}

// slug lowercases s and collapses runs of non-alphanumeric bytes to "-",
// trimmed at both ends. Underscores are preserved so slug(kind+"_"+group)
// keeps the "_" join (pinned by F15-T3 case 8 and the oauth_limits fixture).
func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := true // trims a leading run
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_':
			b.WriteByte(c)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-") // trims a trailing run
}

// pickFinitePercent ports utilization ?? used_percent with finitePercent
// semantics (core.mjs:202-205): a present non-null key wins even when invalid
// (no fallback), null falls through to the next key.
func pickFinitePercent(src map[string]json.RawMessage, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := src[key]
		if !ok || isJSONNull(raw) {
			continue
		}
		return finitePercent(raw)
	}
	return 0, false
}

// pickResetTime ports resets_at ?? reset_at with resetText semantics
// (core.mjs:212-224).
func pickResetTime(src map[string]json.RawMessage, keys ...string) *time.Time {
	for _, key := range keys {
		raw, ok := src[key]
		if !ok || isJSONNull(raw) {
			continue
		}
		return resetTime(raw)
	}
	return nil
}

// finitePercent ports finitePercent (core.mjs:202-205): number or non-empty
// trimmed numeric string; valid iff finite and 0 <= v <= 100.
func finitePercent(raw json.RawMessage) (float64, bool) {
	v, ok := parseNumber(raw)
	if !ok {
		return 0, false
	}
	if v < 0 || v > 100 {
		return 0, false
	}
	return v, true
}

// finiteNonNegative ports finiteNonNegative (core.mjs:207-210): number or
// non-empty trimmed numeric string; valid iff finite and v >= 0.
func finiteNonNegative(raw json.RawMessage) (float64, bool) {
	v, ok := parseNumber(raw)
	if !ok {
		return 0, false
	}
	if v < 0 {
		return 0, false
	}
	return v, true
}

// parseNumber coerces a JSON number or a non-empty trimmed numeric string.
func parseNumber(raw json.RawMessage) (float64, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return 0, false
	}
	switch trimmed[0] {
	case 'n', 't', 'f': // null / true / false
		return 0, false
	case '"':
		var s string
		if json.Unmarshal(trimmed, &s) != nil {
			return 0, false
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	default:
		var v float64
		if json.Unmarshal(trimmed, &v) != nil {
			return 0, false
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	}
}

// resetTime ports resetText (core.mjs:212-224): ISO-8601 string (length <=
// 128) or positive epoch number (> 10_000_000_000 is ms, else seconds);
// returns nil when unparseable.
func resetTime(raw json.RawMessage) *time.Time {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if json.Unmarshal(trimmed, &s) != nil || len(s) > 128 {
			return nil
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil
		}
		return &t
	}
	n, ok := parseNumber(trimmed)
	if !ok || n <= 0 {
		return nil
	}
	if n > 10_000_000_000 {
		t := time.UnixMilli(int64(n)).UTC()
		return &t
	}
	t := time.Unix(int64(n), 0).UTC()
	return &t
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}
