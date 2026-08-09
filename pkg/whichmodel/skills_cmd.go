package whichmodel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/skills"
)

func init() { register(NewSkillsCmd) }

// targetValue implements pflag.Value for --target: "claude"|"generic".
// An unset flag reads as generic (the zero value is normalized in RunE).
type targetValue struct{ v *skills.Target }

func (t *targetValue) String() string {
	if t.v == nil || *t.v == "" {
		return string(skills.TargetGeneric)
	}
	return string(*t.v)
}

func (t *targetValue) Set(s string) error {
	switch skills.Target(s) {
	case skills.TargetGeneric, skills.TargetClaude:
		*t.v = skills.Target(s)
		return nil
	}
	return fmt.Errorf("unknown target: %s", s)
}

func (t *targetValue) Type() string { return "target" }

// NewSkillsCmd installs, removes, or lists agent skills (F28 CONTRACTS
// §4.2). Exit codes: unknown target/name and --user misuse return
// *UsageError (exit 2); filesystem/state errors from internal/skills are
// returned as-is (exit 1).
func NewSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install, remove, or list agent skills",
	}
	cmd.AddCommand(newSkillsInstallCmd(), newSkillsRemoveCmd(), newSkillsListCmd())
	return cmd
}

func newSkillsInstallCmd() *cobra.Command {
	var target skills.Target
	var user, force bool
	var repo string
	cmd := &cobra.Command{
		Use:   "install [name...]",
		Short: "Install agent skills (default: all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo != "" {
				skills.SetRepoDir(repo)
			}
			t := target
			if t == "" {
				t = skills.TargetGeneric
			}
			if t != skills.TargetClaude && user {
				return &UsageError{Message: "--user is only supported with --target claude"}
			}
			names := args
			if len(names) == 0 {
				names = skills.Names
			}
			for _, name := range names {
				if !validSkillName(name) {
					return &UsageError{Message: "unknown skill: " + name + " (known: " + strings.Join(skills.Names, ", ") + ")"}
				}
			}
			for _, name := range names {
				msg, err := skills.Install(name, t, user, force)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
			}
			return nil
		},
	}
	cmd.Flags().Var(&targetValue{&target}, "target", "claude|generic (default generic)")
	cmd.Flags().BoolVar(&user, "user", false, "install into the user skill dir (~/.claude/skills)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite modified destination files")
	cmd.Flags().StringVar(&repo, "repo", "", "repository root (default: nearest .git ancestor)")
	return cmd
}

func newSkillsRemoveCmd() *cobra.Command {
	var target skills.Target
	var user, force bool
	var repo string
	cmd := &cobra.Command{
		Use:   "remove [name...]",
		Short: "Remove installed agent skills (default: all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo != "" {
				skills.SetRepoDir(repo)
			}
			t := target
			if t == "" {
				t = skills.TargetGeneric
			}
			if t != skills.TargetClaude && user {
				return &UsageError{Message: "--user is only supported with --target claude"}
			}
			names := args
			if len(names) == 0 {
				names = skills.Names
			}
			for _, name := range names {
				if !validSkillName(name) {
					return &UsageError{Message: "unknown skill: " + name + " (known: " + strings.Join(skills.Names, ", ") + ")"}
				}
			}
			for _, name := range names {
				msg, err := skills.Remove(name, t, user, force)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
			}
			return nil
		},
	}
	cmd.Flags().Var(&targetValue{&target}, "target", "claude|generic (default generic)")
	cmd.Flags().BoolVar(&user, "user", false, "remove from the user skill dir (~/.claude/skills)")
	cmd.Flags().BoolVar(&force, "force", false, "delete modified installed files")
	cmd.Flags().StringVar(&repo, "repo", "", "repository root (default: nearest .git ancestor)")
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	var target skills.Target
	var user bool
	var repo string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed agent skills",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repo != "" {
				skills.SetRepoDir(repo)
			}
			t := target
			if t == "" {
				t = skills.TargetGeneric
			}
			if t != skills.TargetClaude && user {
				return &UsageError{Message: "--user is only supported with --target claude"}
			}
			names, err := skills.List(t, user)
			if err != nil {
				return err
			}
			if names == nil {
				names = make([]string, 0)
			}
			if jsonOut {
				doc := map[string]any{
					"target":    string(t),
					"user":      user,
					"installed": names,
				}
				b, err := json.Marshal(doc)
				if err != nil {
					return err
				}
				cmd.OutOrStdout().Write(append(b, '\n'))
				return nil
			}
			for _, name := range names {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			return nil
		},
	}
	cmd.Flags().Var(&targetValue{&target}, "target", "claude|generic (default generic)")
	cmd.Flags().BoolVar(&user, "user", false, "list the user skill dir (~/.claude/skills)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, `emit {"target":...,"user":...,"installed":[...]}`)
	cmd.Flags().StringVar(&repo, "repo", "", "repository root (default: nearest .git ancestor)")
	return cmd
}

// validSkillName reports whether name is one of the shipped skill names.
func validSkillName(name string) bool {
	for _, n := range skills.Names {
		if n == name {
			return true
		}
	}
	return false
}
