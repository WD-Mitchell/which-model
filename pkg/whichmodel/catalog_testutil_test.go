package whichmodel

import "testing"

// captureExecuteFresh forces the command registry to rebuild fresh (new
// *cobra.Command + new local flag struct per subcommand) then runs
// captureExecute. Without this, cobra's process-lifetime command cache
// (registry.go) leaks StringArray/scalar flag state across ExecuteArgs
// calls within one test binary — invisible in production (one execution
// per process) but real for in-process tests exercising array-typed
// catalog flags (--provider/--add/--reasoning).
func captureExecuteFresh(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	resetRegistryBuildCache()
	return captureExecute(t, args)
}
