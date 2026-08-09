package whichmodel

import (
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/schema"
)

var (
	schemaMu   sync.RWMutex
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

// NewSchemaCmd prints the JSON Schema documents for command --json outputs
// (F28 CONTRACTS §4.1): no argument → the index; one command name → that
// command's document; unknown name → UnknownCommandError (its message is
// prefixed `unknown command "`, so renderError maps it to exit 2).
func NewSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema [command...]",
		Short: "Print the JSON Schema for a command's --json output",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.OutOrStdout().Write(schema.Index())
				return nil
			}
			doc, err := schema.Emit(args[0])
			if err != nil {
				return err
			}
			cmd.OutOrStdout().Write(doc)
			return nil
		},
	}
}
