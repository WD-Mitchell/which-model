package whichmodel

import (
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

// commandOrder is the fixed display/wiring order (Main DECISION A). Commands
// absent from the list sort last, then alphabetically by name.
var commandOrder = []string{
	"usage", "catalog", "pick", "routes", "auth",
	"schema", "skills", "hooks", "explain", "serve",
	"config", "version",
}

type registrar struct {
	name    string
	factory func() *cobra.Command
}

var (
	registryMu sync.Mutex
	registrars []registrar
	built      []*cobra.Command
	builtCount int
)

// register records one command factory; called from init() in each
// feature-owned <name>_cmd.go. Registration is additive and may also happen
// in tests; the next registeredCommands call observes it.
func register(factory func() *cobra.Command) {
	name := factory().Name()
	registryMu.Lock()
	defer registryMu.Unlock()
	registrars = append(registrars, registrar{name: name, factory: factory})
}

// registeredCommands returns every registered command, stable-sorted by
// commandOrder index (absent names last, then alphabetically). The slice is
// cached and rebuilt only when new registrars appear, so consecutive calls
// without registration return the identical slice.
func registeredCommands() []*cobra.Command {
	registryMu.Lock()
	defer registryMu.Unlock()
	if built != nil && len(registrars) == builtCount {
		return built
	}
	index := make(map[string]int, len(commandOrder))
	for i, name := range commandOrder {
		index[name] = i
	}
	cmds := make([]*cobra.Command, 0, len(registrars))
	for _, r := range registrars {
		cmds = append(cmds, r.factory())
	}
	sort.SliceStable(cmds, func(i, j int) bool {
		ii, okI := index[cmds[i].Name()]
		jj, okJ := index[cmds[j].Name()]
		switch {
		case okI && okJ:
			return ii < jj
		case okI != okJ:
			return okI
		default:
			return cmds[i].Name() < cmds[j].Name()
		}
	})
	built = cmds
	builtCount = len(registrars)
	return built
}
