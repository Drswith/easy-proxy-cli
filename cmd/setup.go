package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drswith/easy-proxy-switch-cli/internal/setup"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install config + shell hooks (detects $SHELL and existing rc files)",
		Args:  cobra.NoArgs,
		Long: `Install default config and inject idempotent shell hooks into rc/profile files.

Detection policy (see also: eps setup --explain):
  1. Prefer $SHELL (login shell) — NOT the interpreter running curl|bash
  2. Also update any existing supported rc/profile files
  3. Include PowerShell profiles when pwsh/powershell is available
  4. Override with repeated --shell flags

After setup, open a new terminal (or source your rc), then: eps on`,
		RunE: func(cmd *cobra.Command, args []string) error {
			explain, _ := cmd.Flags().GetBool("explain")
			if explain {
				fmt.Fprint(os.Stdout, setup.ExplainDetection())
				return nil
			}

			noModify, _ := cmd.Flags().GetBool("no-modify-rc")
			uninstall, _ := cmd.Flags().GetBool("uninstall")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			initCfg, _ := cmd.Flags().GetBool("init-config")
			createMissing, _ := cmd.Flags().GetBool("create-missing")
			shells, _ := cmd.Flags().GetStringSlice("shell")
			bin, _ := cmd.Flags().GetString("bin")

			shells = splitShellFlags(shells)
			if len(shells) == 0 && flagShell != "" {
				shells = []string{flagShell}
			}

			opt := setup.Options{
				Shells:        shells,
				BinPath:       bin,
				AllDetected:   true,
				CreateMissing: createMissing,
				DryRun:        dryRun,
				Uninstall:     uninstall,
				InitConfig:    initCfg && !uninstall,
			}

			if noModify {
				// Config only: use shell=cmd which has no rc targets.
				opt.Shells = []string{"cmd"}
			}

			res, runErr := setup.Run(opt)
			if printErr := printSetupResult(res); printErr != nil {
				return printErr
			}
			return runErr
		},
	}

	cmd.Flags().Bool("explain", false, "print shell detection policy and exit")
	cmd.Flags().Bool("no-modify-rc", false, "only init config; do not edit shell rc/profile files")
	cmd.Flags().Bool("uninstall", false, "remove easy-proxy-switch-cli hook blocks from rc files")
	cmd.Flags().Bool("dry-run", false, "show what would change without writing")
	cmd.Flags().Bool("init-config", true, "create ~/.easy-proxy-switch/config.toml if missing")
	cmd.Flags().Bool("create-missing", true, "create rc/profile files when missing")
	cmd.Flags().StringSlice("shell", nil, "only configure these shells (repeatable): zsh,bash,fish,sh,powershell,nu")
	cmd.Flags().String("bin", "", "eps binary path to embed in hooks (default: this executable)")
	return cmd
}

func splitShellFlags(shells []string) []string {
	var out []string
	for _, s := range shells {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func printSetupResult(res setup.Result) error {
	if flagJSON {
		return out().JSON(res)
	}
	fmt.Fprintf(os.Stdout, "binary: %s\n", res.Binary)
	if res.Config != "" {
		fmt.Fprintf(os.Stdout, "config: %s\n", res.Config)
	}
	if len(res.Targets) == 0 {
		fmt.Fprintln(os.Stdout, "targets: (none)")
	}
	for _, t := range res.Targets {
		line := fmt.Sprintf("[%s] %s (%s) %s", t.Action, t.Target.Path, t.Target.Shell, t.Target.Source)
		if t.Error != "" {
			line += " — " + t.Error
		}
		fmt.Fprintln(os.Stdout, line)
	}
	for _, h := range res.Hints {
		hint("%s", h)
	}
	return nil
}
