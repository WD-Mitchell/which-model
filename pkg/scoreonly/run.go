//go:build nousage

// Package scoreonly implements the restricted offline command surface. It has
// no application file, environment, network, credential or subprocess inputs.
package scoreonly

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/WD-Mitchell/which-model/data"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/WD-Mitchell/which-model/internal/pick"
)

// Version and Commit are stamped by the release build, never runtime settings.
var Version = "dev"
var Commit = "unknown"

const artifact = "which-model-score-only"
const notice = "Score-only recommendation; provider availability and allowances are unverified."
const help = `which-model-score-only — offline ranking with a bundled catalog

Commands:
  pick [--profile balanced_implementation] [--top 3] [--json]
  profiles [--json]
  capabilities [--json]
  version [--json]
  help

No network, credentials, configuration files, integrations or harness execution.
Catalog and profile updates require a new verified artifact.
`

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func manifest() map[string]any {
	// encoding/json orders map keys. These existing decimal profiles contain
	// no unsupported JSON values; the digest identifies the actual weights.
	profiles, err := json.Marshal(pick.Profiles)
	if err != nil {
		panic(err)
	}
	return map[string]any{
		"artifact": artifact, "version": Version, "source_commit": Commit,
		"enabled":  []string{"score_ranking", "bundled_catalog", "builtin_profiles"},
		"excluded": []string{"network", "provider_authentication", "provider_usage", "codexbar", "harness_execution", "skill_installation", "hook_installation", "configuration_files", "persistence"},
		"catalog":  map[string]string{"source_path": "data/available_model_scores.csv", "sha256": hash([]byte(data.ScoresCSV))},
		"profiles": map[string]string{"source_path": "internal/pick/profiles.go", "sha256": hash(profiles)},
	}
}

// Run writes only to the supplied output streams. Return codes follow the
// restricted F21 contract: 0 success, 2 invalid request, 1 data/output failure.
func Run(args []string, stdout, stderr io.Writer) int {
	fail := func(code int, err error) int { fmt.Fprintln(stderr, artifact+":", err); return code }
	text := func(value string) int {
		if _, err := io.WriteString(stdout, value); err != nil {
			return fail(1, err)
		}
		return 0
	}
	if len(args) == 0 {
		return text(help)
	}
	command := args[0]
	switch command {
	case "help", "--help", "-h":
		if len(args) != 1 {
			return fail(2, errors.New("help takes no arguments"))
		}
		return text(help)
	case "pick", "profiles", "capabilities", "version":
	default:
		return fail(2, fmt.Errorf("command %q is unavailable in the restricted distribution", command))
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write JSON")
	var profile *string
	var top *int
	if command == "pick" {
		profile = flags.String("profile", "balanced_implementation", "built-in ranking profile")
		top = flags.Int("top", 3, "total recommendations to display")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return text(help)
		}
		return fail(2, err)
	}
	if flags.NArg() != 0 {
		return fail(2, errors.New("unexpected positional arguments"))
	}
	render := func(payload any) int {
		if err := output.RenderJSON(stdout, output.OutputEnvelope{UsageDisabledReason: "compiled_out"}, payload); err != nil {
			return fail(1, err)
		}
		return 0
	}
	switch command {
	case "capabilities":
		return render(manifest())
	case "version":
		if *jsonOutput {
			return render(map[string]string{"artifact": artifact, "version": Version, "source_commit": Commit})
		}
		return text(fmt.Sprintf("%s %s (%s)\n", artifact, Version, Commit))
	case "profiles":
		names := make([]string, 0, len(pick.Profiles))
		for name := range pick.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		if *jsonOutput {
			return render(map[string]any{"artifact": artifact, "profiles": names})
		}
		for _, name := range names {
			if code := text(name + "\n"); code != 0 {
				return code
			}
		}
		return 0
	}
	p, ok := pick.Profiles[*profile]
	if !ok {
		return fail(2, fmt.Errorf("unknown built-in profile %q", *profile))
	}
	if *top < 1 {
		return fail(2, errors.New("--top must be positive"))
	}
	rows, err := score.ParseScoresCSV([]byte(data.ScoresCSV))
	if err != nil {
		return fail(1, fmt.Errorf("bundled catalog: %w", err))
	}
	ranking, err := pick.Rank(rows, p, nil)
	if err != nil {
		return fail(1, err)
	}
	if len(ranking.Alternatives) > *top-1 {
		ranking.Alternatives = ranking.Alternatives[:*top-1]
	}
	if *jsonOutput {
		m := manifest()
		return render(map[string]any{"artifact": artifact, "notice": notice, "catalog_sha256": m["catalog"].(map[string]string)["sha256"], "profiles_sha256": m["profiles"].(map[string]string)["sha256"], "ranking": ranking})
	}
	if code := text(notice + "\n"); code != 0 {
		return code
	}
	if code := text(fmt.Sprintf("Profile: %s; eligible models: %d\n", ranking.Profile, ranking.CandidateCount)); code != 0 {
		return code
	}
	models := append([]pick.ModelScore{ranking.Recommendation}, ranking.Alternatives...)
	for i, model := range models {
		if code := text(fmt.Sprintf("%d. %s (%s): %s\n", i+1, model.Model, model.Reasoning, model.Total.String())); code != 0 {
			return code
		}
		for _, warning := range model.Warnings {
			if code := text("   Warning: " + warning + "\n"); code != 0 {
				return code
			}
		}
	}
	return 0
}
