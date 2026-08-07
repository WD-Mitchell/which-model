// Package identity provides the catalog's canonical keys: model-name
// cleaning, reasoning-effort parsing and collapse, and benchmark alias keys
// (specs/features/F07-identity/SPEC.md §1). It is a pure leaf — stdlib only,
// no internal imports (specs/global/CONTRACTS.md §8).
package identity

import "strings"

// CleanModelName removes balanced (), [], {} annotation groups from a model
// display name and normalizes whitespace (SPEC.md §2.1, verbatim port of
// clean_model_name, model_types.py:27-59). Total: never errors; may return "".
func CleanModelName(value string) string {
	kept := make([]rune, 0, len(value))
	stack := make([]rune, 0, 4)
	for _, r := range value {
		switch r {
		case '(', '[', '{':
			stack = append(stack, r)
			continue
		case ')', ']', '}':
			var matching rune
			switch r {
			case ')':
				matching = '('
			case ']':
				matching = '['
			case '}':
				matching = '{'
			}
			if len(stack) > 0 && stack[len(stack)-1] == matching {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if len(stack) == 0 {
			kept = append(kept, r)
		}
	}
	return strings.Join(strings.Fields(string(kept)), " ")
}
