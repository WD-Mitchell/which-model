//go:build !nousage

package whichmodel

// wantTreeOrder and helpGoldenPath vary by build tag: usage registers only
// in the default build (F24 command surface).
var wantTreeOrder = []string{"usage", "catalog", "pick", "routes", "schema", "skills", "hooks", "explain", "serve", "config", "version"}

const helpGoldenPath = "testdata/help.golden"
