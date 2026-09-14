package service

import (
	"strings"
	"syscall"

	"github.com/WD-Mitchell/which-model/internal/approvedexec"
)

func displayApprovedCommand(plan approvedexec.Plan) string {
	// PowerShell treats smart single quotes as delimiters too.
	escape := strings.NewReplacer("'", "''", "‘", "‘‘", "’", "’’", "‚", "‚‚", "‛", "‛‛")
	quote := func(value string) string { return "'" + escape.Replace(value) + "'" }
	args := make([]string, len(plan.Args))
	for i, arg := range plan.Args {
		// Match os/exec's Windows command-line serialization, including empty
		// arguments and backslashes before quotes or the closing delimiter.
		args[i] = syscall.EscapeArg(arg)
	}
	// PowerShell's native argument binder differs between 5.1/Legacy and 7.x.
	// ProcessStartInfo bypasses that binder on both .NET Framework and Core.
	// The script block keeps helper variables local; native stdio/environment
	// are inherited from the terminal, and LASTEXITCODE records child status.
	return "& { $start = New-Object System.Diagnostics.ProcessStartInfo; " +
		"$start.FileName = " + quote(plan.Path) + "; " +
		"$start.Arguments = " + quote(strings.Join(args, " ")) + "; " +
		"$start.WorkingDirectory = $ExecutionContext.SessionState.Path.CurrentFileSystemLocation.ProviderPath; " +
		"$start.UseShellExecute = $false; " +
		"$child = [System.Diagnostics.Process]::Start($start); " +
		"$child.WaitForExit(); $global:LASTEXITCODE = $child.ExitCode; $child.Dispose() }"
}
