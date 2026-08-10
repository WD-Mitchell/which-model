//go:build nousage

package whichmodel

// wantTreeOrder and helpGoldenPath vary by build tag: usage is absent from the
// nousage binary (F21 command surface). Kept as test-support globals so
// TestTree/TestHelpGolden share one implementation.
var wantTreeOrder = []string{"catalog", "pick", "routes", "schema", "skills", "hooks", "explain", "serve", "config", "version"}

const helpGoldenPath = "testdata/help_nousage.golden"
