package pick

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Identity is an exact (model, reasoning) tuple as used by the availability
// filter. Matching is exact membership; no cleaning or fuzzy matching.
type Identity struct {
	Model     string
	Reasoning string
}

// ParseAvailability parses a JSON array or plain-text availability list
// (rank_models.py _availability_values + _identity + parse_availability,
// annex-b §5.7). JSON elements: plain string (separator rule), object
// {"model","reasoning"}, or [model, reasoning] pair. Plain text: one identity
// per non-blank non-# line, optional case/space-insensitive header line
// "model,reasoning"|"model|reasoning" skipped. Separator priority
// "|", "::", ",", "/" with last-occurrence split. Empty input returns
// (nil, nil) — the caller treats nil as "no filter supplied".
func ParseAvailability(data []byte) ([]Identity, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	seen := map[Identity]bool{}
	result := []Identity{}
	add := func(id Identity) {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	if trimmed[0] == '[' || trimmed[0] == '{' {
		var entries []json.RawMessage
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, &RankingError{Message: "availability JSON must be a list"}
		}
		for _, entry := range entries {
			id, err := parseAvailabilityEntry(entry)
			if err != nil {
				return nil, err
			}
			add(id)
		}
		return result, nil
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if i == 0 && isAvailabilityHeader(stripped) {
			continue
		}
		id, err := parseIdentity(stripped)
		if err != nil {
			return nil, err
		}
		add(id)
	}
	if len(result) == 0 {
		return nil, &RankingError{Message: "availability list contains no identities"}
	}
	return result, nil
}

// parseAvailabilityEntry decodes one JSON availability element: a plain
// string (separator rule), an object {"model","reasoning"}, or a 2-element
// [model, reasoning] array of strings; anything else is an
// "invalid availability entry: <q>" error (rank_models.py
// _availability_values).
func parseAvailabilityEntry(entry json.RawMessage) (Identity, error) {
	raw := bytes.TrimSpace(entry)
	switch {
	case len(raw) > 0 && raw[0] == '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
		}
		return parseIdentity(s)
	case len(raw) > 0 && raw[0] == '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
		}
		modelRaw, modelOK := obj["model"]
		reasoningRaw, reasoningOK := obj["reasoning"]
		if !modelOK || !reasoningOK || !isJSONString(modelRaw) || !isJSONString(reasoningRaw) {
			return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
		}
		var model, reasoning string
		_ = json.Unmarshal(modelRaw, &model)
		_ = json.Unmarshal(reasoningRaw, &reasoning)
		return Identity{Model: strings.TrimSpace(model), Reasoning: strings.TrimSpace(reasoning)}, nil
	case len(raw) > 0 && raw[0] == '[':
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
		}
		if len(parts) != 2 || !isJSONString(parts[0]) || !isJSONString(parts[1]) {
			return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
		}
		var model, reasoning string
		_ = json.Unmarshal(parts[0], &model)
		_ = json.Unmarshal(parts[1], &reasoning)
		return Identity{Model: strings.TrimSpace(model), Reasoning: strings.TrimSpace(reasoning)}, nil
	}
	return Identity{}, &RankingError{Message: "invalid availability entry: " + string(raw)}
}

// isJSONString reports whether the raw JSON value is a string literal.
func isJSONString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

// parseIdentity ports rank_models.py _identity: separators are tried in
// priority order "|", "::", ",", "/", splitting on the LAST occurrence;
// both halves must be non-blank after trimming or the next separator is
// tried; none usable is the verbatim error.
func parseIdentity(value string) (Identity, error) {
	candidate := strings.TrimSpace(value)
	for _, separator := range []string{"|", "::", ",", "/"} {
		if idx := strings.LastIndex(candidate, separator); idx >= 0 {
			model := strings.TrimSpace(candidate[:idx])
			reasoning := strings.TrimSpace(candidate[idx+len(separator):])
			if model != "" && reasoning != "" {
				return Identity{Model: model, Reasoning: reasoning}, nil
			}
		}
	}
	return Identity{}, &RankingError{
		Message: fmt.Sprintf("availability identity %q must use model|reasoning, model::reasoning, model,reasoning, or model/reasoning", value),
	}
}

// isAvailabilityHeader reports whether a line is the case/space-insensitive
// "model,reasoning" or "model|reasoning" header line.
func isAvailabilityHeader(line string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(line), " ", "")
	return normalized == "model,reasoning" || normalized == "model|reasoning"
}
