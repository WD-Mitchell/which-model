//go:build !nousage

package whichmodel

// wantTreeOrder and helpGoldenPath vary by build tag: usage/auth register
// only in the default build (annex-d §4.6 L2; F24/F25 CONTRACTS "the file is
// build-tagged //go:build !nousage, so a nousage binary does not register the
// command at all"). Kept as test-support globals so TestTree/TestHelpGolden
// (F22-owned) need no per-tag logic of their own.
var wantTreeOrder = []string{"usage", "auth", "schema", "serve", "config", "version"}

const helpGoldenPath = "testdata/help.golden"
