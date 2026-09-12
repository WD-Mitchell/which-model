//go:build !nousage

package codexbar

import (
	"context"
	"io"
	"os/exec"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestCompanyCodexBarProviderSourceLabels(t *testing.T) {
	for _, tc := range []struct {
		provider, label string
		actual, forced  usage.Source
	}{
		{"antigravity", "app", usage.SourceLocal, usage.SourceCLI},
		{"antigravity", "ide", usage.SourceLocal, usage.SourceCLI},
		{"antigravity", "oauth", usage.SourceOAuth, usage.SourceOAuth},
		{"antigravity", "cli", usage.SourceCLI, usage.SourceCLI},
		{"codex", "pat", usage.SourceAPI, usage.SourceAPI},
		{"codex", "openai-web", usage.SourceWeb, usage.SourceWeb},
		{"codex", "codex-cli", usage.SourceCLI, usage.SourceCLI},
		{"claude", "claude", usage.SourceCLI, usage.SourceCLI},
		{"claude", "claude-cli", usage.SourceCLI, usage.SourceCLI},
		{"claude", "admin-api", usage.SourceAPI, usage.SourceAPI},
		{"windsurf", "windsurf-web", usage.SourceWeb, usage.SourceWeb},
		{"windsurf", "local", usage.SourceLocal, usage.SourceCLI},
	} {
		t.Run(tc.provider+"/"+tc.label, func(t *testing.T) {
			p := companyFixture(t)
			p.AllowedProviders = []string{tc.provider}
			for _, requested := range []usage.Source{"", usage.SourceAPI, usage.SourceOAuth, usage.SourceWeb, usage.SourceCLI} {
				runCompanyCommand = func(cmd *exec.Cmd) error {
					want := []string{cmd.Path, "usage", "--provider", tc.provider, "--format", "json", "--json-only", "--no-color"}
					if requested != "" {
						want = append(want, "--source", string(requested))
					}
					if !reflect.DeepEqual(cmd.Args, want) {
						t.Fatalf("requested %q: args=%v", requested, cmd.Args)
					}
					_, err := io.WriteString(cmd.Stdout, companyPayload(tc.provider, tc.label))
					return err
				}
				got, err := FetchWithSource(context.Background(), tc.provider, requested)
				if err != nil {
					t.Fatal(err)
				}
				if requested == "" || requested == tc.forced {
					if got.Failure != nil || !got.UsageKnown || got.Source != tc.actual {
						t.Errorf("requested %q: source=%q failure=%+v", requested, got.Source, got.Failure)
					}
				} else if got.Failure == nil {
					t.Errorf("requested %q accepted %q evidence", requested, got.Source)
				}
			}
		})
	}
}

func TestCompanyCodexBarSourceAliasesStayProviderBound(t *testing.T) {
	for _, label := range []string{"app", "ide", "pat", "openai-web", "codex-cli", "windsurf-web", "offline", "invented-web"} {
		t.Run(label, func(t *testing.T) {
			companyFixture(t)
			runCompanyCommand = func(cmd *exec.Cmd) error {
				_, err := io.WriteString(cmd.Stdout, companyPayload("claude", label))
				return err
			}
			got, err := Fetch(context.Background(), "claude")
			if err != nil || got.Failure == nil {
				t.Fatalf("unreviewed provider/source pair accepted: %+v %v", got, err)
			}
		})
	}
}
