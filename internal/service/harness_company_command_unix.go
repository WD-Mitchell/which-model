//go:build !windows

package service

import (
	"strings"

	"github.com/WD-Mitchell/which-model/internal/approvedexec"
)

func displayApprovedCommand(plan approvedexec.Plan) string {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	words := []string{quote(plan.Path)}
	for _, arg := range plan.Args {
		words = append(words, quote(arg))
	}
	return strings.Join(words, " ")
}
