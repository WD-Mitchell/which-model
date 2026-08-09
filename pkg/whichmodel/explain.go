// F26: explain — history read + Evidence reconstruction
// (specs/features/F26-cmd-pick/SPEC.md §2.10, §2.11, D-11..D-13).
package whichmodel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// RunExplain emits ExplainResult (annex-c §4.3) for the selected history
// record: the last line, or the line whose ULID matches --pick-id. A
// missing/empty history is CodedError no_record (exit 1). Config load
// errors are UsageError (exit 2) (F26 CONTRACTS §2).
func RunExplain(args ExplainArgs, stdout, stderr io.Writer) error {
	if args.Last == (args.PickID != "") {
		return &UsageError{Message: "exactly one of --last or --pick-id is required"}
	}
	cfg, err := config.Load(config.LoadOptions{Path: args.ConfigPath})
	if err != nil {
		return &UsageError{Message: err.Error()}
	}
	path, err := historyPath(cfg)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	entries, err := readHistory(path)
	if err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	var entry *HistoryEntry
	if args.Last {
		if len(entries) == 0 {
			return &CodedError{Code: "no_record", Message: "no record in pick history"}
		}
		entry = &entries[len(entries)-1]
	} else {
		for i := range entries {
			if entries[i].ULID == args.PickID {
				entry = &entries[i]
				break
			}
		}
		if entry == nil {
			return &CodedError{Code: "no_record", Message: fmt.Sprintf("no record %s", args.PickID)}
		}
	}
	res := &ExplainResult{SchemaVersion: "2.0", Candidate: entry.CandidateID, Evidence: entry.Evidence}
	if stdout == nil {
		return nil
	}
	if args.JSON {
		if err := writeJSONDoc(stdout, res); err != nil {
			return &CodedError{Code: "runtime", Message: err.Error()}
		}
		return nil
	}
	if _, err := io.WriteString(stdout, FormatExplainText(*entry)); err != nil {
		return &CodedError{Code: "runtime", Message: err.Error()}
	}
	return nil
}

// readHistory parses every JSONL line of the history file. A missing file
// is an empty history, not an error (first run).
func readHistory(path string) ([]HistoryEntry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]HistoryEntry, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("corrupt history line: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// FormatExplainText renders one history record per CONTRACTS §7. Lines are
// omitted when their fields are absent (degraded mode); candidate_id is
// "-" when the record had no pick.
func FormatExplainText(entry HistoryEntry) string {
	ev := entry.Evidence
	candidate := entry.CandidateID
	if candidate == "" {
		candidate = "-"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "explain %s (%s): picked %s (score %s)\n", ev.Profile, entry.ULID, candidate, formatPickNumber(entry.FinalScore))
	if ev.Confidence != "" {
		fmt.Fprintf(&b, "  confidence: %s\n", ev.Confidence)
	}
	if ev.Band != nil {
		fmt.Fprintf(&b, "  band: %s (%s%% used, weight %s)\n", ev.Band.Name, formatPickNumber(ev.Band.UsedPercent), formatPickNumber(ev.Band.Weight))
	}
	if ev.RouteProvenance != "" {
		fmt.Fprintf(&b, "  route_provenance: %s\n", ev.RouteProvenance)
	}
	for _, x := range ev.ExcludedCandidates {
		fmt.Fprintf(&b, "  excluded: %s (%s)\n", x.Route.Provider+":"+x.Route.ModelID, x.ReasonCode)
	}
	if ev.LastVerified != "" {
		fmt.Fprintf(&b, "  last_verified: %s\n", ev.LastVerified)
	}
	return b.String()
}
