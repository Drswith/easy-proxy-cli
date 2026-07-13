package cmd

import (
	"fmt"
	"os"

	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/proxy"
	"github.com/drswith/easy-proxy-cli/internal/shell"
	"github.com/spf13/cobra"
)

func resolveFlags(cmd *cobra.Command) (proxy.ResolveOptions, error) {
	opt := proxy.ResolveOptions{}
	var err error

	opt.Profile, err = cmd.Flags().GetString("profile")
	if err != nil {
		return opt, err
	}
	opt.HTTP, _ = cmd.Flags().GetString("http")
	opt.HTTPS, _ = cmd.Flags().GetString("https")
	opt.Socks, _ = cmd.Flags().GetString("socks")
	opt.NoProxy, _ = cmd.Flags().GetString("no-proxy")
	opt.Host, _ = cmd.Flags().GetString("host")
	opt.Port, _ = cmd.Flags().GetInt("port")
	opt.Mode, _ = cmd.Flags().GetString("mode")
	opt.DisableNode, _ = cmd.Flags().GetBool("no-node")
	opt.DisableUppercase, _ = cmd.Flags().GetBool("no-uppercase")
	opt.NodeExtraCACerts, _ = cmd.Flags().GetString("node-ca")

	if cmd.Flags().Changed("node") {
		v, _ := cmd.Flags().GetBool("node")
		opt.NodeUseEnvProxy = &v
	}
	if cmd.Flags().Changed("uppercase") {
		v, _ := cmd.Flags().GetBool("uppercase")
		opt.MirrorUppercase = &v
	}
	return opt, nil
}

func addResolveFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("profile", "p", "", "profile name from ~/.easy-proxy/config.toml")
	cmd.Flags().String("http", "", "override HTTP proxy URL")
	cmd.Flags().String("https", "", "override HTTPS proxy URL")
	cmd.Flags().String("socks", "", "override SOCKS/all_proxy URL")
	cmd.Flags().String("no-proxy", "", "override NO_PROXY list")
	cmd.Flags().String("host", "", "shortcut: set host for http+socks (with --port)")
	cmd.Flags().Int("port", 0, "shortcut: set port for http+socks (with --host)")
	cmd.Flags().String("mode", "", "mixed|http|socks (default: mixed)")
	cmd.Flags().Bool("node", true, "set NODE_USE_ENV_PROXY=1")
	cmd.Flags().Bool("no-node", false, "do not set Node.js proxy sugar")
	cmd.Flags().Bool("uppercase", true, "also export HTTP_PROXY/HTTPS_PROXY/...")
	cmd.Flags().Bool("no-uppercase", false, "only lowercase proxy vars")
	cmd.Flags().String("node-ca", "", "set NODE_EXTRA_CA_CERTS path")
}

func newOnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "on",
		Short: "Enable proxy env (prints shell exports; use --emit with eval/hook)",
		Long: `Resolve proxy settings and emit shell export statements on stdout.

Examples:
  eval "$(ezp on --emit)"
  ezp on --emit --host 127.0.0.1 --port 7890
  ezp on --json                 # agent: resolved env as JSON (no shell script)
  ezp on --profile company --emit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			emit, _ := cmd.Flags().GetBool("emit")
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			opt, err := resolveFlags(cmd)
			if err != nil {
				return err
			}
			resolved, err := proxy.Resolve(cfg, opt)
			if err != nil {
				return err
			}

			w := out()
			if flagJSON {
				return w.JSON(map[string]any{
					"action":  "on",
					"profile": resolved.Profile,
					"mode":    resolved.Mode,
					"env":     resolved.Env,
				})
			}

			if !emit {
				hint("tip: run eval \"$(ezp on --emit)\" or install hook: eval \"$(ezp hook zsh)\"")
				hint("enabled profile=%s mode=%s (showing exports; not applied to this process)", resolved.Profile, resolved.Mode)
			}

			kind, err := shell.Detect(flagShell)
			if err != nil {
				return err
			}
			if flagShell == "" {
				kind = shell.Posix
			}
			w.Script(shell.EmitExport(kind, resolved.Env))
			return nil
		},
	}
	addResolveFlags(cmd)
	cmd.Flags().Bool("emit", false, "emit shell script only (for eval/hook; quieter hints)")
	return cmd
}

func newOffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "off",
		Short: "Disable proxy env (prints shell unset statements)",
		RunE: func(cmd *cobra.Command, args []string) error {
			emit, _ := cmd.Flags().GetBool("emit")
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			noNode, _ := cmd.Flags().GetBool("no-node")
			noUpper, _ := cmd.Flags().GetBool("no-uppercase")
			mirror := cfg.Extras.MirrorUppercase && !noUpper
			node := cfg.Extras.NodeUseEnvProxy && !noNode
			keys := proxy.OffKeys(mirror, node)

			w := out()
			if flagJSON {
				return w.JSON(map[string]any{
					"action": "off",
					"unset":  keys,
				})
			}
			if !emit {
				hint("tip: run eval \"$(ezp off --emit)\" or use the shell hook")
			}
			kind, err := shell.Detect(flagShell)
			if err != nil {
				return err
			}
			if flagShell == "" {
				kind = shell.Posix
			}
			w.Script(shell.EmitUnset(kind, keys))
			return nil
		},
	}
	cmd.Flags().Bool("emit", false, "emit shell script only (for eval/hook)")
	cmd.Flags().Bool("no-node", false, "do not unset Node.js vars")
	cmd.Flags().Bool("no-uppercase", false, "do not unset uppercase vars")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show currently set proxy-related environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			cur := proxy.CurrentFromOS()
			w := out()
			if flagJSON {
				return w.JSON(map[string]any{
					"active":  len(cur) > 0,
					"env":     cur,
					"count":   len(cur),
				})
			}
			if len(cur) == 0 {
				fmt.Fprintln(os.Stdout, "proxy: inactive (no managed vars set in this process)")
				return nil
			}
			fmt.Fprintln(os.Stdout, "proxy: active")
			for _, k := range proxy.SortedKeys(cur) {
				fmt.Fprintf(os.Stdout, "  %s=%s\n", k, cur[k])
			}
			return nil
		},
	}
}

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print resolved env without applying (export|json|dotenv)",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			opt, err := resolveFlags(cmd)
			if err != nil {
				return err
			}
			resolved, err := proxy.Resolve(cfg, opt)
			if err != nil {
				return err
			}
			w := out()
			switch format {
			case "json":
				return w.JSON(resolved.Env)
			case "dotenv":
				for _, k := range proxy.SortedKeys(resolved.Env) {
					fmt.Fprintf(os.Stdout, "%s=%s\n", k, resolved.Env[k])
				}
			case "export", "":
				kind, err := shell.Detect(flagShell)
				if err != nil {
					return err
				}
				if flagShell == "" {
					kind = shell.Posix
				}
				w.Script(shell.EmitExport(kind, resolved.Env))
			default:
				return fmt.Errorf("unknown format %q (export|json|dotenv)", format)
			}
			return nil
		},
	}
	addResolveFlags(cmd)
	cmd.Flags().String("format", "export", "export|json|dotenv")
	return cmd
}

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec -- <command> [args...]",
		Short: "Run a command with proxy env applied (no shell eval needed)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			opt, err := resolveFlags(cmd)
			if err != nil {
				return err
			}
			resolved, err := proxy.Resolve(cfg, opt)
			if err != nil {
				return err
			}
			environ := proxy.ApplyToEnviron(os.Environ(), resolved.Env)
			c := execCommand(args[0], args[1:]...)
			c.Env = environ
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				if code := exitCode(err); code >= 0 {
					os.Exit(code)
				}
				return fmt.Errorf("exec %q: %w", args[0], err)
			}
			return nil
		},
	}
	addResolveFlags(cmd)
	return cmd
}
