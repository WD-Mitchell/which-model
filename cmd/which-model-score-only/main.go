//go:build nousage

package main

import (
	"os"

	"github.com/WD-Mitchell/which-model/pkg/scoreonly"
)

func main() { os.Exit(scoreonly.Run(os.Args[1:], os.Stdout, os.Stderr)) }
