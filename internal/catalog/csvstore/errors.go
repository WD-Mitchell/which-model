// Package csvstore provides atomic CSV persistence for the catalog: bounded
// reads, atomic writes, timestamped backups with rotation, identity-keyed
// merging, and raw-CSV provenance hashing (specs/features/F06-csvstore/SPEC.md
// §1). This package MUST NOT import internal/catalog/identity or anything
// under internal/usage, internal/routing, or internal/pick
// (specs/global/CONTRACTS.md §8).
package csvstore

import "errors"

var (
	ErrMissingFile        = errors.New("csv file missing")
	ErrFileTooLarge       = errors.New("csv file too large")
	ErrMalformedCSV       = errors.New("malformed csv")
	ErrDuplicateIdentity  = errors.New("duplicate model/reasoning identity")
	ErrChangedDuringWrite = errors.New("csv file changed while data was being collected")
)
