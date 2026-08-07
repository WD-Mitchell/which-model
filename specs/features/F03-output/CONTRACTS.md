---
kind: feature-contracts
feature: F03-output
version: "1.0"
project: which-model
---

# F03 — output: CONTRACTS

Package `internal/output`, files under `internal/output/`. Imports: Go stdlib only (`encoding/json`, `fmt`, `io`, `strings`, `errors`) — Layer 0 (`specs/global/SPEC.md` §3). All functions are safe for concurrent use (no shared mutable state).

**Rule (binding, every command):** all `--json` output MUST be produced by `RenderJSON`, so every emitted JSON document carries the `OutputEnvelope` fields `schema_version`, `usage_enabled`, and (when non-empty) `usage_disabled_reason`. No command may emit JSON by another path.

## 1. Output envelope (canonical — `specs/global/CONTRACTS.md` §6)

File: `internal/output/envelope.go`

```go
package output

// OutputEnvelope is the canonical envelope carried by every --json document.
// Defined in specs/global/CONTRACTS.md §6; F03 owns the Go type.
type OutputEnvelope struct {
    SchemaVersion       string `json:"schema_version"` // "2.0"
    UsageEnabled        bool   `json:"usage_enabled"`
    UsageDisabledReason string `json:"usage_disabled_reason,omitempty"` // flag|config|compiled_out|no_providers_enabled
}

// SchemaVersion is the current JSON schema version (specs/global/CONTRACTS.md §7).
const SchemaVersion = "2.0"
```

## 2. JSON renderer

File: `internal/output/json.go`

```go
// RenderJSON marshals payload, injects the envelope fields at the top level of
// the resulting JSON object, writes the deterministic document (sorted keys,
// precision-preserving UseNumber round-trip) followed by "\n", and returns any
// writer error. env.SchemaVersion defaults to SchemaVersion when empty.
// ErrPayloadNotObject: payload does not marshal to a JSON object.
// ErrReservedField: payload already contains schema_version / usage_enabled /
// usage_disabled_reason.
func RenderJSON(w io.Writer, env OutputEnvelope, payload any) error

// ErrReservedField: payload already contains an envelope key; the key is named
// in the error message.
var ErrReservedField = errors.New("output: payload uses a reserved envelope field")

// ErrPayloadNotObject: payload does not marshal to a JSON object.
var ErrPayloadNotObject = errors.New("output: payload must be a JSON object")
```

## 3. Text renderers

File: `internal/output/text.go`

```go
// RenderLines writes each line followed by "\n"; empty input writes nothing.
func RenderLines(w io.Writer, lines []string) error

// RenderTable writes an aligned table: header row first, each column padded to
// the width of its widest cell (header included), cells joined with one space,
// each row newline-terminated. Rows shorter than the header are padded with
// empty cells; a row longer than the header returns an error.
func RenderTable(w io.Writer, headers []string, rows [][]string) error
```

## 4. Stream discipline helpers (stdout = payload, stderr = warnings + failure)

File: `internal/output/stream.go`

```go
// WriteFailure writes the fixed failure line
// "which-model <command>: [<code>] <message>\n" (annex-d §1.3). Callers route
// it to stderr.
func WriteFailure(w io.Writer, command, code, message string) error

// WriteWarning writes "warning: <message>\n" (annex-d §1.3). Callers route it
// to stderr.
func WriteWarning(w io.Writer, message string) error

// RedactIdentity implements the --show-identity contract: returns nil when
// show is false (the value is omitted entirely, never masked — annex-d §1.2),
// or a pointer to value when show is true. JSON callers use omitempty; text
// renderers skip nil values.
func RedactIdentity(value string, show bool) *string
```

## 5. Schema printer

File: `internal/output/schema.go`

```go
// PrintSchema writes one JSON Schema document (deterministic marshal of doc,
// sorted keys) followed by "\n" (annex-d §2.9).
func PrintSchema(w io.Writer, doc map[string]any) error

// PrintSchemaIndex writes {"commands": [...]} followed by "\n" for
// `which-model schema` with no argument (annex-d §2.9). commands is emitted in
// the order given.
func PrintSchemaIndex(w io.Writer, commands []string) error
```

## 6. Ownership

- **Flags owned:** none. Global flags `--json`, `--schema`, `--show-identity`, `--quiet`, `--verbose`, `--no-color` are declared and wired by F22-cli-skeleton; F03 provides the renderers and the identity hook.
- **Config keys owned:** none.
- **Error codes added:** none (`specs/global/CONTRACTS.md` §1.6 table is unchanged). `ErrReservedField`/`ErrPayloadNotObject` are Go sentinels for programmer errors, not `Failure.Code` values.
- **JSON shapes emitted:** every `--json` document (envelope + payload per §2); the schema index `{"commands": [...]}` (§5). Command payload shapes are owned by the emitting feature (F23/F24/F26/...).
- **Exit codes:** none directly; commands map F03 errors to runtime errors (exit 1, `specs/global/SPEC.md` §5).
