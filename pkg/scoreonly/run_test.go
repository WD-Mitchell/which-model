//go:build nousage

package scoreonly

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/data"
	"github.com/WD-Mitchell/which-model/internal/catalog/score"
	"github.com/WD-Mitchell/which-model/internal/pick"
)

func invoke(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	code := Run(args, &out, &err)
	return code, out.String(), err.String()
}

func TestBundledRankingMatchesExistingEngine(t *testing.T) {
	rows, err := score.ParseScoresCSV([]byte(data.ScoresCSV))
	if err != nil {
		t.Fatal(err)
	}
	for name, profile := range pick.Profiles {
		t.Run(name, func(t *testing.T) {
			want, err := pick.Rank(rows, profile, nil)
			if err != nil {
				t.Fatal(err)
			}
			code, out, stderr := invoke(t, "pick", "--profile", name, "--top", "99999", "--json")
			if code != 0 {
				t.Fatalf("%d: %s", code, stderr)
			}
			var doc struct {
				Ranking      json.RawMessage
				UsageEnabled bool   `json:"usage_enabled"`
				Reason       string `json:"usage_disabled_reason"`
				Notice       string
			}
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatal(err)
			}
			expected, _ := json.Marshal(want)
			var gotValue, wantValue any
			json.Unmarshal(doc.Ranking, &gotValue)
			json.Unmarshal(expected, &wantValue)
			gotBytes, _ := json.Marshal(gotValue)
			wantBytes, _ := json.Marshal(wantValue)
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Fatal("ranking differs from existing engine")
			}
			if doc.UsageEnabled || doc.Reason != "compiled_out" || !strings.Contains(doc.Notice, "unverified") {
				t.Fatal("missing score-only boundary")
			}
			_, again, _ := invoke(t, "pick", "--profile", name, "--top", "99999", "--json")
			if out != again {
				t.Fatal("output is not deterministic")
			}
		})
	}
}

func TestExcludedCommandsAndOverrides(t *testing.T) {
	for _, args := range [][]string{
		{"usage"}, {"auth", "login"}, {"catalog", "refresh"}, {"routes"}, {"run"}, {"launch"},
		{"skill", "install"}, {"skills", "install"}, {"hook", "install"}, {"hooks", "install"}, {"codexbar"},
		{"pick", "--usage"}, {"pick", "--config", "untrusted.toml"}, {"pick", "--harness", "custom"},
		{"pick", "--catalog", "evil.csv"}, {"pick", "--profile", "unknown"}, {"pick", "--top", "0"},
		{"pick", "--top", "-1"}, {"pick", "extra"}, {"profiles", "--usage"}, {"version", "extra"}, {"help", "usage"},
	} {
		code, out, err := invoke(t, args...)
		if code != 2 || out != "" || err == "" {
			t.Errorf("%v: code=%d out=%q err=%q", args, code, out, err)
		}
	}
}

func TestManifestAndTop(t *testing.T) {
	for _, command := range []string{"capabilities", "version", "profiles"} {
		code, out, err := invoke(t, command, "--json")
		if code != 0 || !json.Valid([]byte(out)) || err != "" {
			t.Fatalf("%s: %d %s %s", command, code, out, err)
		}
	}
	code, out, err := invoke(t, "pick", "--top", "1", "--json")
	if code != 0 {
		t.Fatal(err)
	}
	var doc struct{ Ranking pick.Result }
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Ranking.Alternatives) != 0 || doc.Ranking.CandidateCount < 2 {
		t.Fatal("top truncation changed candidate count")
	}
	for _, args := range [][]string{nil, {"--help"}, {"help"}, {"pick", "--help"}} {
		code, out, _ := invoke(t, args...)
		if code != 0 || !strings.Contains(out, "which-model-score-only") {
			t.Errorf("help %v", args)
		}
	}
}
