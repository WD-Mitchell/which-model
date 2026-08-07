package output

import (
	"encoding/json"
	"io"
)

// PrintSchema writes one JSON Schema document (deterministic marshal of doc,
// sorted keys) followed by "\n" (annex-d §2.9).
func PrintSchema(w io.Writer, doc map[string]any) error {
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// PrintSchemaIndex writes {"commands": [...]} followed by "\n" for
// `which-model schema` with no argument (annex-d §2.9). commands is emitted in
// the order given.
func PrintSchemaIndex(w io.Writer, commands []string) error {
	b, err := json.Marshal(struct {
		Commands []string `json:"commands"`
	}{Commands: commands})
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}
