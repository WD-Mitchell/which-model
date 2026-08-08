package whichmodel

import (
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/output"
)

var (
	schemaMu  sync.RWMutex
	schemaDocs = map[string]map[string]any{}
)

func init() { register(NewSchemaCmd) }

// RegisterSchema records the JSON Schema document for a command's --json
// output (cmdPath e.g. "version", "config show"). Last write wins.
func RegisterSchema(cmdPath string, doc map[string]any) {
	copied := make(map[string]any, len(doc))
	for k, v := range doc {
		copied[k] = v
	}
	schemaMu.Lock()
	defer schemaMu.Unlock()
	schemaDocs[cmdPath] = copied
}

func lookupSchema(cmdPath string) (map[string]any, bool) {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	doc, ok := schemaDocs[cmdPath]
	return doc, ok
}

// SchemaIndex lists registered command paths, sorted.
func SchemaIndex() []string {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	keys := make([]string, 0, len(schemaDocs))
	for k := range schemaDocs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NewSchemaCmd prints schema documents (SPEC §9): no argument → the index;
// one argument → that command's document; unknown → UsageError (exit 2).
func NewSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [command]",
		Short: "print the JSON schema of a command's output",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return output.PrintSchemaIndex(Stdout, SchemaIndex())
			}
			doc, ok := lookupSchema(args[0])
			if !ok {
				return &UsageError{Message: fmt.Sprintf("no schema for command %q", args[0])}
			}
			return output.PrintSchema(Stdout, doc)
		},
	}
}
