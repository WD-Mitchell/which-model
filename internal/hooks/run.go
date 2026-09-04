package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Envelope is the sole stdout shape (SPEC behaviour 3).
type Envelope struct {
	Decision           string         `json:"decision"`
	Reason             string         `json:"reason,omitempty"`
	HookSpecificOutput map[string]any `json:"hookSpecificOutput"`
}

// MarshalEnvelope is compact JSON + "\n".
func MarshalEnvelope(e Envelope) []byte {
	b, err := json.Marshal(e)
	if err != nil {
		b = []byte(`{"decision":"approve","reason":"internal marshal error"}`)
	}
	return append(b, '\n')
}

// Runner executes the underlying command in-process.
type Runner func(args []string, stdout, stderr io.Writer) int

// Options carries the test seams (SPEC behaviour 4).
type Options struct {
	Runner   Runner
	Stdin    []byte
	Env      map[string]string
	RepoRoot string
}

var (
	errUnknownHook = errors.New("unknown hook")
	errBadStdin    = errors.New("stdin is not valid JSON object")
)

// Run executes hook. Returns stdout bytes (possibly empty = fail-open
// silence) or an error for exit-2-class conditions. Never errors for
// underlying command failures (fail-open).
func Run(name string, passthrough []string, opts Options) ([]byte, error) {
	h, ok := Get(name)
	if !ok {
		return nil, errUnknownHook
	}
	input := bytes.TrimSpace(opts.Stdin)
	if len(input) > 0 && (!json.Valid(input) || input[0] != '{') {
		return nil, errBadStdin
	}
	runner := opts.Runner
	if runner == nil {
		// A missing command runner is an underlying failure, never success.
		runner = func([]string, io.Writer, io.Writer) int { return 1 }
	}
	var stdout, stderr bytes.Buffer
	code := runner(h.Underlying(passthrough, opts.Env), &stdout, &stderr)
	out := stdout.Bytes()
	return dispatch(h, code, out, opts)
}

// dispatch interprets the underlying run per hook (SPEC behaviours 5–8).
func dispatch(h Hook, code int, out []byte, opts Options) ([]byte, error) {
	switch h.ID {
	case "usage-refresh":
		if code != 0 {
			return nil, nil // fail-open silence (SPEC behaviour 6)
		}
		return MarshalEnvelope(Envelope{
			Decision:           "approve",
			Reason:             "usage cache refreshed",
			HookSpecificOutput: map[string]any{},
		}), nil
	case "quota-guard":
		if code != 0 {
			return nil, nil // fail-open silence
		}
		var doc struct {
			Snapshots []struct {
				Provider string `json:"provider"`
			} `json:"snapshots"`
		}
		if err := json.Unmarshal(out, &doc); err != nil || doc.Snapshots == nil {
			return nil, nil // unparseable → fail-open silence
		}
		seen := map[string]bool{}
		var providers []string
		for _, s := range doc.Snapshots {
			if s.Provider != "" && !seen[s.Provider] {
				seen[s.Provider] = true
				providers = append(providers, s.Provider)
			}
		}
		if len(providers) == 0 {
			return MarshalEnvelope(Envelope{
				Decision:           "approve",
				Reason:             "no providers at or above critical band",
				HookSpecificOutput: map[string]any{},
			}), nil
		}
		return MarshalEnvelope(Envelope{
			Decision: "block",
			Reason:   fmt.Sprintf("quota guard: %d provider(s) at or above critical band", len(providers)),
			HookSpecificOutput: map[string]any{
				"critical_providers": providers,
			},
		}), nil
	case "spawn-gate":
		if code != 0 {
			if code == 4 {
				var doc struct {
					Excluded []json.RawMessage `json:"excluded_candidates"`
				}
				if err := json.Unmarshal(out, &doc); err != nil {
					return approveFailOpen("spawn-gate", 4), nil
				}
				var names []string
				for _, raw := range doc.Excluded {
					var ex struct {
						ReasonCode string `json:"reason_code"`
						Route      struct {
							Provider string `json:"provider"`
						} `json:"route"`
					}
					if err := json.Unmarshal(raw, &ex); err != nil {
						continue
					}
					if ex.ReasonCode == "band_gated" {
						names = append(names, ex.Route.Provider)
					}
				}
				if len(names) == 0 {
					return approveFailOpen("spawn-gate", 4), nil
				}
				return MarshalEnvelope(Envelope{
					Decision: "block",
					Reason:   "all eligible providers band-gated: " + strings.Join(names, ", "),
					HookSpecificOutput: map[string]any{
						"excluded_candidates": doc.Excluded,
					},
				}), nil
			}
			return approveFailOpen("spawn-gate", code), nil
		}
		var doc struct {
			Candidates []json.RawMessage `json:"candidates"`
		}
		if err := json.Unmarshal(out, &doc); err != nil || len(doc.Candidates) == 0 {
			return approveFailOpen("spawn-gate", 0), nil
		}
		var first struct {
			CandidateID string `json:"candidate_id"`
		}
		if err := json.Unmarshal(doc.Candidates[0], &first); err != nil {
			return approveFailOpen("spawn-gate", 0), nil
		}
		return MarshalEnvelope(Envelope{
			Decision: "approve",
			Reason:   "dispatch approved: " + first.CandidateID,
			HookSpecificOutput: map[string]any{
				"candidate": json.RawMessage(doc.Candidates[0]),
			},
		}), nil
	case "model-audit":
		if code != 0 {
			return approveFailOpen("model-audit", code), nil
		}
		var doc auditDocument
		if err := json.Unmarshal(out, &doc); err != nil || doc.SchemaVersion != "2.0" || doc.Evidence == nil {
			return approveFailOpen("model-audit", 0), nil
		}
		provider, modelID, ok := strings.Cut(doc.Candidate, ":")
		if !ok || provider == "" || modelID == "" {
			return approveFailOpen("model-audit", 0), nil
		}
		if expected := envOr(opts.Env, "WHICH_MODEL_CANDIDATE_ID", ""); expected != "" && expected != doc.Candidate {
			return approveFailOpen("model-audit", 0), nil
		}
		// Decode only documented fields, then emit one compact JSONL record.
		// Unrelated host/provider fields must never enter dispatch evidence.
		out, err := json.Marshal(doc)
		if err != nil {
			return approveFailOpen("model-audit", 0), nil
		}
		root := opts.RepoRoot
		if root == "" {
			return approveFailOpen("model-audit", 0), nil
		}
		evidenceDir := filepath.Join(root, ".which-model")
		if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
			return approveFailOpen("model-audit", 0), nil
		}
		evidenceFile := filepath.Join(evidenceDir, "evidence.jsonl")
		if err := appendLine(evidenceFile, out); err != nil {
			return approveFailOpen("model-audit", 0), nil
		}
		mismatch := false
		if dispatched := envOr(opts.Env, "WHICH_MODEL_DISPATCHED_MODEL", ""); dispatched != "" && dispatched != modelID {
			mismatch = true
			rec := map[string]any{
				"ts":               time.Now().UTC().Format(time.RFC3339),
				"dispatched_model": dispatched,
				"route_model_id":   modelID,
				"evidence":         json.RawMessage(out),
			}
			b, err := json.Marshal(rec)
			if err == nil {
				appendLine(filepath.Join(evidenceDir, "audit-mismatches.jsonl"), b)
			}
		}
		return MarshalEnvelope(Envelope{
			Decision: "approve",
			Reason:   "dispatch evidence recorded",
			HookSpecificOutput: map[string]any{
				"evidence_logged": evidenceFile,
				"mismatch":        mismatch,
			},
		}), nil
	}
	return nil, errUnknownHook
}

// approveFailOpen is the fail-open envelope for dispatch-boundary hooks
// (SPEC behaviour 6).
func approveFailOpen(hook string, code int) []byte {
	return MarshalEnvelope(Envelope{
		Decision:           "approve",
		Reason:             fmt.Sprintf("fail-open: %s underlying command exited %d", hook, code),
		HookSpecificOutput: map[string]any{},
	})
}

// appendLine appends one line to path (O_APPEND single write, mode 0600).
func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line = append(line, '\n')
	}
	_, err = f.Write(line)
	return err
}
