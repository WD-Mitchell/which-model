package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrReservedField: payload already contains an envelope key; the key is named
// in the error message.
var ErrReservedField = errors.New("output: payload uses a reserved envelope field")

// ErrPayloadNotObject: payload does not marshal to a JSON object.
var ErrPayloadNotObject = errors.New("output: payload must be a JSON object")

// RenderJSON marshals payload, injects the envelope fields at the top level of
// the resulting JSON object, writes the deterministic document (sorted keys,
// precision-preserving UseNumber round-trip) followed by "\n", and returns any
// writer error. env.SchemaVersion defaults to SchemaVersion when empty.
func RenderJSON(w io.Writer, env OutputEnvelope, payload any) error {
	if env.SchemaVersion == "" {
		env.SchemaVersion = SchemaVersion
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return ErrPayloadNotObject
		}
		return err
	}
	if m == nil {
		return ErrPayloadNotObject
	}
	for _, key := range []string{"schema_version", "usage_enabled", "usage_disabled_reason"} {
		if _, ok := m[key]; ok {
			return fmt.Errorf("%w: %s", ErrReservedField, key)
		}
	}
	m["schema_version"] = env.SchemaVersion
	m["usage_enabled"] = env.UsageEnabled
	if env.UsageDisabledReason != "" {
		m["usage_disabled_reason"] = env.UsageDisabledReason
	}
	out, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}
