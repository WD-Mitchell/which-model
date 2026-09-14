//go:build nousage

// Package data supplies the reviewed source snapshot to the restricted CLI.
package data

import _ "embed"

// ScoresCSV is embedded at build time; no runtime catalog path is accepted.
//
//go:embed available_model_scores.csv
var ScoresCSV string
