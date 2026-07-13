package cmd

import (
	"fmt"
	"os"

	"github.com/drswith/easy-proxy-cli/internal/apperr"
	"github.com/drswith/easy-proxy-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagJSON  bool
	flagShell string
	flagQuiet bool
	version   = "0.1.0"
	commit    = "dev"
	// processExit is os.Exit by default; tests may override to avoid killing the test binary.
	processExit = os.Exit
)

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "ezp",
		Short: "Easy temporary proxy environment variables (agent-first CLI)",
		Long: `ezp quickly enables/disables proxy-related environment variables.

IMPORTANT: a child process cannot export into your current shell.
Use one of:
  eval "$(ezp on --emit)"
  eval "$(ezp hook zsh)"   # then plain: ezp on / ezp off
  ezp exec -- curl https://example.com

Designed for AI agents and humans: stable exit codes, --json, schema.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (%s)", version, commit),
	}

	root.PersistentFlags().BoolVarP(&flagJSON, "json", "j", false, "machine-readable JSON on stdout")
	root.PersistentFlags().StringVar(&flagShell, "shell", "", "shell dialect: bash|zsh|sh|fish|powershell|cmd (auto for emit)")
	root.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress human hints on stderr")

	root.AddCommand(
		newOnCmd(),
		newOffCmd(),
		newStatusCmd(),
		newEnvCmd(),
		newExecCmd(),
		newConfigCmd(),
		newSetupCmd(),
		newHookCmd(),
		newDoctorCmd(),
		newProfilesCmd(),
		newSchemaCmd(),
		newCompletionCmd(),
	)

	return root
}

func Execute() {
	if err := ExecuteArgs(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeFor(err))
	}
}

// ExecuteArgs runs the CLI with the given args (no process exit). Used by tests.
func ExecuteArgs(args []string) error {
	resetFlags()
	root := newRoot()
	root.SetArgs(args)
	return root.Execute()
}

func exitCodeFor(err error) int {
	if apperr.IsMisconfig(err) {
		return output.ExitMisconfig
	}
	return output.ExitError
}

// ExitCodeFor maps an error to the process exit code (exported for tests).
func ExitCodeFor(err error) int { return exitCodeFor(err) }

func resetFlags() {
	flagJSON = false
	flagShell = ""
	flagQuiet = false
}

func out() output.Writer {
	return output.New(output.Mode{JSON: flagJSON})
}

func hint(format string, args ...any) {
	if flagQuiet || flagJSON {
		return
	}
	out().Human(format, args...)
}
