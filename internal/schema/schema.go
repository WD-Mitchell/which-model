// Package schema owns the machine contract documents for the CLI's --json
// surfaces (F28 agent skills): the JSON Schema documents for the usage, pick,
// explain, and routes outputs, plus the no-argument index. It imports only
// the Go standard library (specs/global/CONTRACTS.md §8).
package schema

var commandOrder = []string{"usage", "pick", "explain", "routes"}

// Commands returns the schema-bearing commands in index order.
func Commands() []string {
	out := make([]string, len(commandOrder))
	copy(out, commandOrder)
	return out
}

var docs = map[string][]byte{
	"usage":   []byte(usageSchemaJSON),
	"pick":    []byte(pickSchemaJSON),
	"explain": []byte(explainSchemaJSON),
	"routes":  []byte(routesSchemaJSON),
}

// Emit returns the JSON Schema document for name, or an error naming the
// valid commands if name is unknown.
func Emit(name string) ([]byte, error) {
	if d, ok := docs[name]; ok {
		return d, nil
	}
	return nil, &UnknownCommandError{Name: name, Commands: Commands()}
}

// Index returns the no-argument index document (compact JSON + "\n").
func Index() []byte {
	return []byte(`{"commands":["usage","pick","explain","routes"]}` + "\n")
}

// UnknownCommandError names the requested command and the valid commands.
type UnknownCommandError struct {
	Name     string
	Commands []string
}

func (e *UnknownCommandError) Error() string {
	return "unknown command \"" + e.Name + "\" (known: " + joinComma(e.Commands) + ")"
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
