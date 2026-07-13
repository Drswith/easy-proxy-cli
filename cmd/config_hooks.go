package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/apperr"
	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read/write ~/.easy-proxy/config.toml",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create default config if missing",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			path, err := config.Path()
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); err == nil && !force {
				if flagJSON {
					return out().JSON(map[string]any{"created": false, "path": path})
				}
				hint("already exists: %s (use --force to overwrite)", path)
				return nil
			}
			cfg := config.Default()
			if err := config.Save(cfg); err != nil {
				return err
			}
			if flagJSON {
				return out().JSON(map[string]any{"created": true, "path": path})
			}
			fmt.Println(path)
			return nil
		},
	}
	initCmd.Flags().Bool("force", false, "overwrite existing config")

	cmd.AddCommand(
		&cobra.Command{
			Use:   "path",
			Short: "Print config file path",
			RunE: func(cmd *cobra.Command, args []string) error {
				p, err := config.Path()
				if err != nil {
					return err
				}
				if flagJSON {
					return out().JSON(map[string]string{"path": p})
				}
				fmt.Println(p)
				return nil
			},
		},
		&cobra.Command{
			Use:   "show",
			Short: "Show current config (creates default if missing)",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.LoadOrCreate()
				if err != nil {
					return err
				}
				if flagJSON {
					return out().JSON(cfg)
				}
				path, _ := config.Path()
				hint("config: %s", path)
				data, err := encodeTOML(cfg)
				if err != nil {
					return err
				}
				fmt.Print(data)
				return nil
			},
		},
		initCmd,
		&cobra.Command{
			Use:   "set-default <profile>",
			Short: "Set default profile name",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.LoadOrCreate()
				if err != nil {
					return err
				}
				if _, err := cfg.Profile(args[0]); err != nil {
					return err
				}
				cfg.DefaultProfile = args[0]
				if err := config.Save(cfg); err != nil {
					return err
				}
				if flagJSON {
					return out().JSON(map[string]string{"default_profile": args[0]})
				}
				fmt.Println(args[0])
				return nil
			},
		},
		&cobra.Command{
			Use:   "set <profile> <key> <value>",
			Short: "Set a profile field (http|https|socks|no_proxy)",
			Args:  cobra.ExactArgs(3),
			RunE: func(cmd *cobra.Command, args []string) error {
				name, key, value := args[0], strings.ToLower(args[1]), args[2]
				cfg, err := config.LoadOrCreate()
				if err != nil {
					return err
				}
				p, ok := cfg.Profiles[name]
				if !ok {
					p = config.Profile{}
					if cfg.Profiles == nil {
						cfg.Profiles = map[string]config.Profile{}
					}
				}
				switch key {
				case "http":
					p.HTTP = value
				case "https":
					p.HTTPS = value
				case "socks", "all_proxy", "all":
					p.Socks = value
				case "no_proxy", "noproxy":
					p.NoProxy = value
				default:
					return apperr.Misconfigf("unknown key %q (http|https|socks|no_proxy)", key)
				}
				cfg.Profiles[name] = p
				if err := config.Save(cfg); err != nil {
					return err
				}
				if flagJSON {
					return out().JSON(map[string]any{"profile": name, "key": key, "value": value})
				}
				fmt.Printf("%s.%s=%s\n", name, key, value)
				return nil
			},
		},
	)
	return cmd
}

func encodeTOML(cfg config.Config) (string, error) {
	// Lazy import via Save's marshal — duplicate small helper
	return marshalConfig(cfg)
}

func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <shell>",
		Short: "Print shell integration so `ezp on|off` mutates the current shell",
		Long: `Add to your shell rc:

  # zsh
  eval "$(ezp hook zsh)"

  # bash
  eval "$(ezp hook bash)"

  # fish
  ezp hook fish | source

Then: ezp on / ezp off work without eval.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := shellDetect(args[0])
			if err != nil {
				return err
			}
			bin, err := os.Executable()
			if err != nil {
				bin = "ezp"
			}
			script, err := hookScript(kind, bin)
			if err != nil {
				return err
			}
			if flagJSON {
				return out().JSON(map[string]string{"shell": args[0], "script": script})
			}
			fmt.Print(script)
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check whether the configured proxy is reachable",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			opt, err := resolveFlags(cmd)
			if err != nil {
				return err
			}
			resolved, err := resolveProxy(cfg, opt)
			if err != nil {
				return err
			}
			probe, _ := cmd.Flags().GetString("url")
			if probe == "-" {
				probe = ""
			}
			report := runDoctor(resolved.Env["http_proxy"], resolved.Env["all_proxy"], probe)
			w := out()
			if flagJSON {
				if err := w.JSON(report); err != nil {
					return err
				}
				if !report.OK {
					processExit(output.ExitUnavailable)
				}
				return nil
			}
			for _, c := range report.Checks {
				status := "ok"
				if !c.OK {
					status = "FAIL"
				}
				line := fmt.Sprintf("[%s] %s %s", status, c.Name, c.Target)
				if c.Latency != "" {
					line += " (" + c.Latency + ")"
				}
				if c.Error != "" {
					line += " — " + c.Error
				}
				fmt.Fprintln(os.Stdout, line)
			}
			if !report.OK {
				processExit(output.ExitUnavailable)
			}
			return nil
		},
	}
	addResolveFlags(cmd)
	cmd.Flags().String("url", "https://www.google.com", "URL to probe via HTTP proxy (use '-' to skip)")
	return cmd
}

func newProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "profiles",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrCreate()
			if err != nil {
				return err
			}
			if flagJSON {
				return out().JSON(map[string]any{
					"default":  cfg.DefaultProfile,
					"profiles": cfg.Profiles,
				})
			}
			for _, name := range cfg.ListProfiles() {
				mark := " "
				if name == cfg.DefaultProfile {
					mark = "*"
				}
				p := cfg.Profiles[name]
				fmt.Printf("%s %-16s http=%s socks=%s\n", mark, name, p.HTTP, p.Socks)
			}
			return nil
		},
	}
}

func newSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print machine-readable command/config schema (for AI agents)",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema := map[string]any{
				"name":        "ezp",
				"version":     version,
				"config_path": "~/.easy-proxy/config.toml",
				"exit_codes": map[string]int{
					"ok":          output.ExitOK,
					"error":       output.ExitError,
					"misconfig":   output.ExitMisconfig,
					"unavailable": output.ExitUnavailable,
				},
				"conventions": map[string]string{
					"stdout":      "shell scripts, JSON payloads, command results",
					"stderr":      "human hints (suppressed by --json / --quiet)",
					"shell_apply": "eval \"$(ezp on --emit)\" or eval \"$(ezp hook <shell>)\"",
				},
				"commands": []map[string]any{
					{"name": "on", "desc": "emit exports", "agent": "ezp on --json | ezp on --emit"},
					{"name": "off", "desc": "emit unset", "agent": "ezp off --json | ezp off --emit"},
					{"name": "status", "desc": "current process env", "agent": "ezp status --json"},
					{"name": "env", "desc": "resolved env dump", "agent": "ezp env --format=json"},
					{"name": "exec", "desc": "run child with proxy", "agent": "ezp exec -- curl -I https://example.com"},
					{"name": "doctor", "desc": "reachability", "agent": "ezp doctor --json"},
					{"name": "config", "desc": "read/write config", "agent": "ezp config show --json"},
					{"name": "profiles", "desc": "list profiles", "agent": "ezp profiles --json"},
					{"name": "setup", "desc": "install config + shell hooks", "agent": "ezp setup --json"},
					{"name": "hook", "desc": "shell integration", "agent": "ezp hook zsh"},
					{"name": "schema", "desc": "this schema", "agent": "ezp schema"},
				},
				"config": config.Default(),
				"shells": []string{"bash", "zsh", "sh", "fish", "powershell", "nu"},
			}
			return out().JSON(schema)
		},
	}
}

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return cmd
}
