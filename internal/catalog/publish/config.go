package publish

import (
	"fmt"
	"github.com/WD-Mitchell/which-model/internal/catalog"
	"strconv"
	"strings"
)

// ValidationError is a [catalog.publish] validation failure. It exposes
// ExitCode() == 2 so the CLI maps it to exit 2 (code "config") without
// inventing a new failure code.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func (e *ValidationError) ExitCode() int { return 2 }

// PublishConfig mirrors [catalog.publish] plus the raw CSV path staged by the
// generated workflow.
type PublishConfig = catalog.PublishConfig

// Defaults (annex-b §8.1; SPEC behaviour 2).
const (
	DefaultSchedule      = "0 6 * * *"
	DefaultTimezone      = "Europe/London"
	DefaultMode          = "pull-request"
	DefaultMergeMethod   = "squash"
	DefaultCommitMessage = "chore(data): refresh available model scores"
	DefaultPRTitle       = "chore(data): refresh available model scores"
)

var DefaultBranches = []string{"main"}
var DefaultPRLabels = []string{"data", "automated"}

// UnmarshalKeyer is satisfied by *config.Config (F01 pin:
// func (c *Config) UnmarshalKey(key string, out any) error).
type UnmarshalKeyer interface {
	UnmarshalKey(key string, out any) error
}

// NewDefaults returns a PublishConfig with every publishing default applied.
// Slice fields are independent copies of the package vars.
func NewDefaults() *PublishConfig {
	return &PublishConfig{
		Enabled:       true,
		Schedule:      DefaultSchedule,
		Timezone:      DefaultTimezone,
		Branches:      append([]string(nil), DefaultBranches...),
		Mode:          DefaultMode,
		AutoMerge:     true,
		MergeMethod:   DefaultMergeMethod,
		CommitMessage: DefaultCommitMessage,
		PRTitle:       DefaultPRTitle,
		PRLabels:      append([]string(nil), DefaultPRLabels...),
	}
}

// Load decodes the complete [catalog] schema, applies publishing defaults for
// absent keys, and runs Validate. Missing section = all defaults. Returns typed
// errors (→ exit 2).
func Load(cfg UnmarshalKeyer) (*PublishConfig, error) {
	cc := catalog.Config{Publish: *NewDefaults()}
	if err := cfg.UnmarshalKey("catalog", &cc); err != nil {
		return nil, err
	}
	pc := &cc.Publish
	pc.RawCSVPath = firstNonEmpty(cc.RawCSVPath, "data/available_model_raw_values.csv")
	if err := Validate(pc); err != nil {
		return nil, err
	}
	return pc, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// Validate checks mode/merge_method/branches/schedule/labels per SPEC
// behaviour 3. Typed errors name the offending key.
func Validate(pc *PublishConfig) error {
	switch pc.Mode {
	case "pull-request", "direct-push":
	default:
		return &ValidationError{Message: fmt.Sprintf("catalog.publish.mode: unknown mode %q (known: pull-request, direct-push)", pc.Mode)}
	}
	switch pc.MergeMethod {
	case "squash", "merge", "rebase":
	default:
		return &ValidationError{Message: fmt.Sprintf("catalog.publish.merge_method: unknown merge method %q (known: squash, merge, rebase)", pc.MergeMethod)}
	}
	if len(pc.Branches) == 0 {
		return &ValidationError{Message: "catalog.publish.branches must not be empty"}
	}
	if err := ValidateCron(pc.Schedule); err != nil {
		return fmt.Errorf("catalog.publish.schedule: %w", err)
	}
	pc.PRLabels = dedupeStrings(pc.PRLabels)
	return nil
}

// dedupeStrings removes duplicate entries preserving first-occurrence order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// cronField describes one of the five cron fields and its bounds.
type cronField struct {
	name    string
	min, ma int
}

var cronFields = []cronField{
	{"minute", 0, 59},
	{"hour", 0, 23},
	{"day-of-month", 1, 31},
	{"month", 1, 12},
	{"day-of-week", 0, 6},
}

// monthNames and dowNames resolve 3-letter English names to their numeric
// value (JAN=1..DEC=12, SUN=0..SAT=6), case-insensitively.
var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

// ValidateCron implements the SPEC behaviour-3 grammar: exactly 5 fields
// (minute, hour, day-of-month, month, day-of-week); per-field tokens `*`,
// a number, `A-B`, `*/N`, `A-B/N`, or comma-lists of those; bounds
// minute 0-59, hour 0-23, dom 1-31, month 1-12, dow 0-6; 3-letter month
// (JAN-DEC) and day (SUN-SAT) names allowed as single tokens or list
// elements, case-insensitive, never inside ranges/steps. Rejects 6-field
// crons, @-keywords, empty fields, out-of-bounds numbers, step 0.
func ValidateCron(schedule string) error {
	fields := strings.Fields(schedule)
	if len(fields) != 5 {
		return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: expected 5 fields, got %d", schedule, len(fields))}
	}
	for i, field := range fields {
		f := cronFields[i]
		for _, el := range strings.Split(field, ",") {
			if el == "" {
				return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: empty list element (%s)", schedule, f.name)}
			}
			if el == "*" {
				continue
			}
			if isNameElement(i, el) {
				continue
			}
			if n, err := strconv.Atoi(el); err == nil {
				if n < f.min || n > f.ma {
					return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: %d out of range %d-%d (%s)", schedule, n, f.min, f.ma, f.name)}
				}
				continue
			}
			if err := validateRangeOrStep(schedule, f, el); err != nil {
				return err
			}
		}
	}
	return nil
}

// isNameElement reports whether el is a bare 3-letter month/day name valid
// for field i. Names are never allowed inside ranges or steps.
func isNameElement(i int, el string) bool {
	upper := strings.ToUpper(el)
	if i == 3 {
		_, ok := monthNames[upper]
		return ok
	}
	if i == 4 {
		_, ok := dowNames[upper]
		return ok
	}
	return false
}

// validateRangeOrStep handles `A-B`, `A-B/N`, and `*/N` elements.
func validateRangeOrStep(schedule string, f cronField, el string) error {
	base := el
	if idx := strings.IndexByte(el, '/'); idx >= 0 {
		base = el[:idx]
		n, err := strconv.Atoi(el[idx+1:])
		if err != nil {
			return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: invalid step %q (%s)", schedule, el[idx+1:], f.name)}
		}
		if n < 1 {
			return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: step 0 (step must be >= 1) (%s)", schedule, f.name)}
		}
	}
	if base == "*" {
		return nil
	}
	idx := strings.IndexByte(base, '-')
	if idx < 0 {
		return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: unknown token %q (%s)", schedule, el, f.name)}
	}
	a, errA := strconv.Atoi(base[:idx])
	b, errB := strconv.Atoi(base[idx+1:])
	if errA != nil || errB != nil {
		return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: name %q not allowed in a range/step (%s)", schedule, el, f.name)}
	}
	for _, n := range []int{a, b} {
		if n < f.min || n > f.ma {
			return &ValidationError{Message: fmt.Sprintf("invalid cron schedule %q: %d out of range %d-%d (%s)", schedule, n, f.min, f.ma, f.name)}
		}
	}
	return nil
}
