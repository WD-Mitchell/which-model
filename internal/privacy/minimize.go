package privacy

import (
	"regexp"
	"strings"
	"time"
)

// Operational identifiers may contain provider model separators but not account
// emails, whitespace, query strings or control characters. These are identifiers,
// not an anonymization promise for user-chosen profile/model aliases.
var operationalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:/+-]{0,255}$`)

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func identifier(v any) string {
	s, _ := v.(string)
	if !operationalID.MatchString(s) || strings.Contains(s, "..") || strings.Contains(s, "://") {
		return ""
	}
	return s
}
func copyID(out, in map[string]any, keys ...string) {
	for _, k := range keys {
		if v := identifier(in[k]); v != "" {
			out[k] = v
		}
	}
}
func copyNumber(out, in map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := in[k].(float64); ok {
			out[k] = v
		}
	}
}
func copyBool(out, in map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := in[k].(bool); ok {
			out[k] = v
		}
	}
}
func copyTime(out, in map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := in[k].(string); ok {
			if _, err := time.Parse(time.RFC3339Nano, v); err == nil {
				out[k] = v
			}
		}
	}
}
func copyEnum(out, in map[string]any, key string, values ...string) {
	v, _ := in[key].(string)
	for _, allowed := range values {
		if v == allowed {
			out[key] = v
			return
		}
	}
}

func evidence(in map[string]any) map[string]any {
	out := map[string]any{"score_inputs": map[string]any{}, "excluded_candidates": []any{}}
	copyID(out, in, "profile")
	copyNumber(out, in, "snapshot_age_seconds")
	copyTime(out, in, "last_verified")
	copyEnum(out, in, "confidence", "live", "cached", "estimated")
	copyEnum(out, in, "route_provenance", "provider_live", "models_dev", "user_declared")
	for key, value := range object(in["score_inputs"]) {
		if identifier(key) != "" {
			if number, ok := value.(float64); ok {
				out["score_inputs"].(map[string]any)[key] = number
			}
		}
	}
	if band := object(in["band"]); band != nil {
		b := map[string]any{}
		copyID(b, band, "name")
		copyNumber(b, band, "used_percent", "weight")
		if len(b) > 0 {
			out["band"] = b
		}
	}
	if excluded, ok := in["excluded_candidates"].([]any); ok {
		for _, raw := range excluded {
			item := object(raw)
			clean := map[string]any{}
			copyEnum(clean, item, "reason_code", "band_gated", "no_score_row", "auth_required", "provider_error", "not_in_availability_list")
			if code, ok := clean["reason_code"]; ok {
				clean["reason"] = code
			} // omit provider error prose
			route := map[string]any{}
			copyID(route, object(item["route"]), "provider", "model_id", "reasoning")
			if model, ok := object(item["route"])["model"].(string); ok && identifier(strings.ReplaceAll(model, " ", "_")) != "" {
				route["model"] = model
			}
			route["window_ids"] = idList(object(item["route"])["window_ids"])
			clean["route"] = route
			if len(clean) > 1 {
				out["excluded_candidates"] = append(out["excluded_candidates"].([]any), clean)
			}
		}
	}
	return out
}
func idList(v any) []any {
	out := []any{}
	if items, ok := v.([]any); ok {
		for _, item := range items {
			if id := identifier(item); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func minimize(category Category, in map[string]any, identityFree bool) (map[string]any, bool) {
	out := map[string]any{}
	switch category {
	case History:
		copyID(out, in, "ulid", "profile", "strategy", "candidate_id")
		if out["profile"] == nil {
			return nil, false
		}
		if out["candidate_id"] == nil {
			out["candidate_id"] = ""
		}
		copyNumber(out, in, "final_score", "excluded_count")
		out["evidence"] = evidence(object(in["evidence"]))
	case Audit:
		copyID(out, in, "candidate", "dispatched_model", "route_model_id")
		inner := object(in["evidence"])
		if nested := object(inner["evidence"]); nested != nil {
			copyID(out, inner, "candidate")
			inner = nested
		}
		if out["candidate"] == nil && out["route_model_id"] == nil {
			return nil, false
		}
		out["schema_version"] = "2.0"
		out["evidence"] = evidence(inner)
	case Launch:
		copyID(out, in, "harness", "provider", "model_id", "profile")
		copyEnum(out, in, "outcome", "started", "copied", "failed")
		if out["outcome"] == nil {
			return nil, false
		}
	case Usage:
		snap := object(in["snapshot"])
		if snap == nil {
			return nil, false
		}
		clean := map[string]any{}
		copyID(clean, snap, "provider")
		if clean["provider"] == nil {
			return nil, false
		}
		if !identityFree {
			for _, k := range []string{"account", "plan"} {
				if v, ok := snap[k].(string); ok {
					clean[k] = v
				}
			}
		}
		copyTime(clean, snap, "fetched_at")
		copyBool(clean, snap, "usage_known", "stale")
		copyEnum(clean, snap, "source", "oauth", "api", "cli", "web", "local", "cache")
		copyEnum(clean, snap, "confidence", "live", "cached", "estimated")
		windows := []any{}
		if list, ok := snap["windows"].([]any); ok {
			for _, v := range list {
				w := object(v)
				next := map[string]any{}
				copyID(next, w, "id")
				if next["id"] == nil {
					continue
				}
				next["label"] = next["id"]
				if !identityFree {
					for _, k := range []string{"label", "reset_hint"} {
						if v, ok := w[k].(string); ok {
							next[k] = v
						}
					}
				}
				copyEnum(next, w, "unit", "percent", "tokens", "credits", "usd", "requests", "kwh", "none")
				copyNumber(next, w, "used_percent", "used", "limit", "remaining", "window_minutes")
				copyBool(next, w, "unlimited", "synthetic", "usage_known")
				copyTime(next, w, "resets_at")
				next["model_scope"] = idList(w["model_scope"])
				windows = append(windows, next)
			}
		}
		clean["windows"] = windows
		if len(windows) == 0 {
			clean["usage_known"] = false
		}
		out["snapshot"] = clean
	default:
		return nil, false
	}
	return out, true
}
