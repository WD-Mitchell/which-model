package pick

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog"
)

// requiredTier1Axes is the exact tier-1 key set a profile must declare.
var requiredTier1Axes = []string{"intelligence", "cost", "speed"}

// ValidateProfile enforces the 6 rules of rank_models.py:80-103 verbatim
// (annex-b §5.2). Returns *RankingError on the first violation.
func ValidateProfile(p catalog.Profile) error {
	// Rule 1: share signs.
	if p.Tier1Share.Sign() <= 0 || p.Tier2Share.Sign() < 0 {
		return &RankingError{Message: "tier 1 share must be positive and tier 2 share cannot be negative"}
	}
	// Rule 2: shares sum to exactly 100.
	if !p.Tier1Share.Add(p.Tier2Share).Equal(decimal.NewFromInt(100)) {
		return &RankingError{Message: "tier 1 and tier 2 shares must sum to 100"}
	}
	// Rule 3: tier-1 keys exactly {intelligence, cost, speed}; missing and
	// unknown both named, each detail sorted, joined with "; ", prefix
	// always lists all three axes.
	required := map[string]bool{}
	for _, axis := range requiredTier1Axes {
		required[axis] = true
	}
	var missing, unknown []string
	for _, axis := range requiredTier1Axes {
		if _, ok := p.Tier1Weights[axis]; !ok {
			missing = append(missing, axis)
		}
	}
	for key := range p.Tier1Weights {
		if !required[key] {
			unknown = append(unknown, key)
		}
	}
	if len(missing) > 0 || len(unknown) > 0 {
		sort.Strings(missing)
		sort.Strings(unknown)
		var details []string
		if len(missing) > 0 {
			details = append(details, "missing "+strings.Join(missing, ", "))
		}
		if len(unknown) > 0 {
			details = append(details, "unknown "+strings.Join(unknown, ", "))
		}
		return &RankingError{
			Message: "tier 1 weights must include intelligence, cost, and speed (" + strings.Join(details, "; ") + ")",
		}
	}
	// Rule 4: tier-1 weights in (0, 5].
	for axis, weight := range p.Tier1Weights {
		if weight.Sign() <= 0 || weight.GreaterThan(decimal.NewFromInt(5)) {
			return &RankingError{Message: "tier 1 weight " + axis + " must be greater than 0 and at most 5"}
		}
	}
	// Rule 5: tier-2 keys must be CategoryNames members (sorted).
	categoryNames := map[string]bool{}
	for _, name := range CategoryNames {
		categoryNames[name] = true
	}
	var unknownCategories []string
	for key := range p.Tier2Weights {
		if !categoryNames[key] {
			unknownCategories = append(unknownCategories, key)
		}
	}
	if len(unknownCategories) > 0 {
		sort.Strings(unknownCategories)
		return &RankingError{Message: "unknown tier 2 categories: " + strings.Join(unknownCategories, ", ")}
	}
	// Rule 6: tier-2 weights in (0, 5].
	for name, weight := range p.Tier2Weights {
		if weight.Sign() <= 0 || weight.GreaterThan(decimal.NewFromInt(5)) {
			return &RankingError{Message: "tier 2 weight " + name + " must be greater than 0 and at most 5"}
		}
	}
	return nil
}

// ProfileFromJSON parses a custom profile from flat or nested JSON
// (rank_models.py profile_from_json): flat keys tier1_share/tier2_share/
// tier1_weights/tier2_weights, or nested tier1:{share,weights|axis...}/
// tier2:{share,weights|category...}; share defaults 100/0. Validates via
// ValidateProfile before returning. Profile.Name is "custom".
func ProfileFromJSON(data []byte) (catalog.Profile, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return catalog.Profile{}, &RankingError{Message: "weights JSON is invalid: " + err.Error()}
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return catalog.Profile{}, &RankingError{Message: "weights JSON must be an object"}
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &doc); err != nil {
		return catalog.Profile{}, &RankingError{Message: "weights JSON is invalid: " + err.Error()}
	}

	// Nested tier1/tier2 objects win over flat keys; a non-object tierN is
	// ignored (flat path), matching rank_models.py profile_from_json.
	tier1Weights := doc["tier1_weights"]
	tier2Weights := doc["tier2_weights"]
	tier1Share := doc["tier1_share"]
	tier2Share := doc["tier2_share"]
	if t1, ok := doc["tier1"]; ok {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(t1, &obj); err == nil && obj != nil {
			if w, ok := obj["weights"]; ok {
				tier1Weights = w
			} else {
				axis := make(map[string]json.RawMessage, len(obj))
				for k, v := range obj {
					if k != "share" {
						axis[k] = v
					}
				}
				rebuilt, err := json.Marshal(axis)
				if err != nil {
					return catalog.Profile{}, &RankingError{Message: "weights JSON is invalid: " + err.Error()}
				}
				tier1Weights = rebuilt
			}
			if s, ok := obj["share"]; ok {
				tier1Share = s
			}
		}
	}
	if t2, ok := doc["tier2"]; ok {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(t2, &obj); err == nil && obj != nil {
			if w, ok := obj["weights"]; ok {
				tier2Weights = w
			} else {
				cats := make(map[string]json.RawMessage, len(obj))
				for k, v := range obj {
					if k != "share" {
						cats[k] = v
					}
				}
				rebuilt, err := json.Marshal(cats)
				if err != nil {
					return catalog.Profile{}, &RankingError{Message: "weights JSON is invalid: " + err.Error()}
				}
				tier2Weights = rebuilt
			}
			if s, ok := obj["share"]; ok {
				tier2Share = s
			}
		}
	}

	// Weights maps: absent key means {}; present but not an object errors.
	weights1, err := parseWeightsObject(tier1Weights, "tier 1")
	if err != nil {
		return catalog.Profile{}, err
	}
	weights2, err := parseWeightsObject(tier2Weights, "tier 2")
	if err != nil {
		return catalog.Profile{}, err
	}

	share1, err := parseDecimalValue(tier1Share, decimal.NewFromInt(100), "tier1 share")
	if err != nil {
		return catalog.Profile{}, err
	}
	share2, err := parseDecimalValue(tier2Share, decimal.NewFromInt(0), "tier2 share")
	if err != nil {
		return catalog.Profile{}, err
	}

	p := catalog.Profile{
		Name:         "custom",
		Tier1Share:   share1,
		Tier2Share:   share2,
		Tier1Weights: weights1,
		Tier2Weights: weights2,
	}
	if err := ValidateProfile(p); err != nil {
		return catalog.Profile{}, err
	}
	return p, nil
}

// parseWeightsObject decodes a weights JSON object into decimal weights,
// applying the _weights checks of rank_models.py: non-blank names, numeric,
// finite, and within [0, 5]. An absent key (nil raw) means an empty object.
func parseWeightsObject(raw json.RawMessage, tier string) (map[string]decimal.Decimal, error) {
	if raw == nil {
		return map[string]decimal.Decimal{}, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, &RankingError{Message: "weights JSON tier1/tier2 weights must be objects"}
	}
	result := make(map[string]decimal.Decimal, len(entries))
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil, &RankingError{Message: tier + " weight names must be non-blank strings"}
		}
		number, err := parseDecimalValue(entries[name], decimal.Zero, tier+" weight "+name)
		if err != nil {
			return nil, err
		}
		if number.LessThan(decimal.Zero) || number.GreaterThan(decimal.NewFromInt(5)) {
			return nil, &RankingError{Message: tier + " weight " + name + " must be between 0 and 5"}
		}
		result[name] = number
	}
	return result, nil
}

// parseDecimalValue ports rank_models.py _decimal: a JSON number or numeric
// string is parsed without float64 round-tripping; unparseable values give
// "<field> must be numeric" and the non-finite tokens NaN/sNaN/Infinity/Inf
// (any sign/case) give "<field> must be finite". An absent key returns the
// fallback (share defaults).
func parseDecimalValue(raw json.RawMessage, fallback decimal.Decimal, field string) (decimal.Decimal, error) {
	if raw == nil {
		return fallback, nil
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 0 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return decimal.Zero, &RankingError{Message: field + " must be numeric"}
		}
		s = str
	}
	if isNonFiniteToken(s) {
		return decimal.Zero, &RankingError{Message: field + " must be finite"}
	}
	number, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, &RankingError{Message: field + " must be numeric"}
	}
	return number, nil
}

// isNonFiniteToken reports whether s is a Python-Decimal non-finite literal
// (NaN, sNaN, Infinity/Inf) with optional sign, any case.
func isNonFiniteToken(s string) bool {
	token := strings.TrimSpace(s)
	if strings.HasPrefix(token, "+") || strings.HasPrefix(token, "-") {
		token = token[1:]
	}
	switch strings.ToLower(token) {
	case "nan", "snan", "inf", "infinity":
		return true
	}
	return false
}
