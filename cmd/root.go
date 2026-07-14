package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drswith/easy-proxy-switch-cli/internal/apperr"
	"github.com/drswith/easy-proxy-switch-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagJSON  bool
	flagShell string
	flagQuiet bool
	version   = "0.1.0"
	commit    = "dev"
)

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "eps",
		Short: "Easy proxy on/off for shell env (agent-first CLI)",
		Long: `eps quickly enables/disables proxy-related environment variables.

IMPORTANT: a child process cannot export into your current shell.
Use one of:
  eval "$(eps on --emit)"
  eval "$(eps hook zsh)"   # then plain: eps on / eps off
  eps exec -- curl https://example.com

Designed for AI agents and humans: stable exit codes, --json, schema.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (%s)", version, commit),
	}

	root.PersistentFlags().BoolVarP(&flagJSON, "json", "j", false, "machine-readable JSON on stdout")
	root.PersistentFlags().StringVar(&flagShell, "shell", "", "shell dialect: bash|zsh|sh|fish|powershell|cmd (auto for emit)")
	root.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress human hints on stderr")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return apperr.Misconfig(err)
	})

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
		// ExitCodeError already streamed output (doctor/exec); skip duplicate stderr.
		if _, ok := apperr.ExitCode(err); !ok {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(exitCodeFor(err))
	}
}

// ExecuteArgs runs the CLI with the given args (no process exit). Used by tests.
func ExecuteArgs(args []string) error {
	resetFlags()
	root := newRoot()
	root.SetArgs(args)
	return classifyCLIError(root.Execute())
}

func classifyCLIError(err error) error {
	if err == nil || apperr.IsMisconfig(err) {
		return err
	}
	if _, ok := apperr.ExitCode(err); ok {
		return err
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unknown flag"),
		strings.Contains(msg, "unknown shorthand"),
		strings.Contains(msg, "unknown command"),
		strings.Contains(msg, "invalid argument"),
		strings.Contains(msg, "accepts "),
		strings.Contains(msg, "requires "),
		strings.Contains(msg, "arg(s)"):
		return apperr.Misconfig(err)
	default:
		return err
	}
}

func exitCodeFor(err error) int {
	if code, ok := apperr.ExitCode(err); ok {
		return code
	}
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
