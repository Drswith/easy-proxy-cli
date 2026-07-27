package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/drswith/easy-proxy-switch-cli/internal/config"
	"github.com/drswith/easy-proxy-switch-cli/internal/shell"
)

// DiscoverTargets decides which rc/profile files to modify.
//
// Policy (auto mode):
//  1. Prefer the login shell from $SHELL (never trust that curl|bash means bash).
//  2. Also include any *existing* supported rc/profile files for other shells
//     (dual-shell users keep both in sync).
//  3. If PowerShell/pwsh is on PATH, include the default profile path for this OS.
//  4. Explicit --shell / Options.Shells overrides auto discovery (only those).
func DiscoverTargets(opt Options) ([]Target, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	if len(opt.Shells) > 0 {
		var out []Target
		for _, name := range opt.Shells {
			kind, err := shell.Detect(name)
			if err != nil {
				return nil, err
			}
			for _, path := range primaryPaths(kind, home) {
				out = append(out, Target{
					Shell:   kind,
					Path:    path,
					Source:  "explicit",
					Existed: fileExists(path),
				})
			}
		}
		return dedupeTargets(out), nil
	}

	byPath := map[string]Target{}

	// 1) login shell: $SHELL, then OS account lookup (self-implemented; no third-party lib)
	if kind, ok := kindFromSHELL(loginShell()); ok {
		for _, path := range primaryPaths(kind, home) {
			byPath[path] = Target{
				Shell:   kind,
				Path:    path,
				Source:  "shell_env",
				Existed: fileExists(path),
			}
		}
	}

	// 2) existing rc files
	for _, c := range candidateFiles(home) {
		if !fileExists(c.Path) {
			continue
		}
		if _, exists := byPath[c.Path]; exists {
			continue
		}
		byPath[c.Path] = Target{
			Shell:   c.Shell,
			Path:    c.Path,
			Source:  "existing_rc",
			Existed: true,
		}
	}

	// 3) PowerShell: always on Windows; on Unix only if profile already exists
	//    (avoid creating unused profiles just because pwsh is installed).
	for _, path := range primaryPaths(shell.PowerShell, home) {
		if _, exists := byPath[path]; exists {
			continue
		}
		ex := fileExists(path)
		if runtime.GOOS == "windows" || ex {
			byPath[path] = Target{
				Shell:   shell.PowerShell,
				Path:    path,
				Source:  "powershell_path",
				Existed: ex,
			}
		}
	}

	out := make([]Target, 0, len(byPath))
	for _, t := range byPath {
		out = append(out, t)
	}
	if len(out) == 0 {
		// last resort: assume zsh on darwin, bash elsewhere
		fallback := shell.Bash
		if runtime.GOOS == "darwin" {
			fallback = shell.Zsh
		}
		for _, path := range primaryPaths(fallback, home) {
			out = append(out, Target{
				Shell:   fallback,
				Path:    path,
				Source:  "fallback",
				Existed: fileExists(path),
			})
		}
	}
	return dedupeTargets(out), nil
}

func loginShell() string {
	if v := strings.TrimSpace(os.Getenv("SHELL")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		if hasCmd("pwsh") {
			if p, err := exec.LookPath("pwsh"); err == nil {
				return p
			}
		}
		if hasCmd("powershell") {
			if p, err := exec.LookPath("powershell"); err == nil {
				return p
			}
		}
		return ""
	}
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		return ""
	}
	// Linux / most Unix: getent passwd
	if out, err := exec.Command("getent", "passwd", username).Output(); err == nil {
		// name:x:uid:gid:gecos:home:shell
		parts := strings.Split(strings.TrimSpace(string(out)), ":")
		if len(parts) >= 7 && parts[6] != "" {
			return parts[6]
		}
	}
	// macOS: dscl
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			out, err := exec.Command("dscl", ".", "-read", home, "UserShell").Output()
			if err == nil {
				line := strings.TrimSpace(string(out))
				if strings.HasPrefix(line, "UserShell:") {
					return strings.TrimSpace(strings.TrimPrefix(line, "UserShell:"))
				}
			}
		}
	}
	return ""
}

func kindFromSHELL(v string) (shell.Kind, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	// Normalize Windows paths when running tests / cross checks on Unix.
	normalized := strings.ReplaceAll(v, `\`, `/`)
	base := strings.ToLower(filepath.Base(normalized))
	base = strings.TrimSuffix(base, ".exe")
	switch base {
	case "zsh":
		return shell.Zsh, true
	case "bash":
		return shell.Bash, true
	case "fish":
		return shell.Fish, true
	case "sh", "dash", "ash":
		return shell.Posix, true
	case "pwsh", "powershell":
		return shell.PowerShell, true
	case "nu", "nushell":
		return shell.Nu, true
	default:
		return "", false
	}
}

type candidate struct {
	Shell shell.Kind
	Path  string
}

func candidateFiles(home string) []candidate {
	cfg := xdgConfigHome(home)
	fish := filepath.Join(cfg, "fish", "config.fish")
	nu := filepath.Join(cfg, "nushell", "config.nu")
	list := []candidate{
		{shell.Zsh, zshrcPath(home)},
		{shell.Bash, filepath.Join(home, ".bashrc")},
		{shell.Bash, filepath.Join(home, ".bash_profile")},
		{shell.Posix, filepath.Join(home, ".profile")},
		{shell.Fish, fish},
		{shell.Nu, nu},
	}
	list = append(list, powershellCandidates(home)...)
	return list
}

func primaryPaths(kind shell.Kind, home string) []string {
	switch kind {
	case shell.Zsh:
		return []string{zshrcPath(home)}
	case shell.Bash:
		// macOS login bash often reads .bash_profile; Linux interactive uses .bashrc.
		if runtime.GOOS == "darwin" {
			return []string{
				filepath.Join(home, ".bash_profile"),
				filepath.Join(home, ".bashrc"),
			}
		}
		return []string{filepath.Join(home, ".bashrc")}
	case shell.Fish:
		return []string{filepath.Join(xdgConfigHome(home), "fish", "config.fish")}
	case shell.Posix:
		return []string{filepath.Join(home, ".profile")}
	case shell.Nu:
		return []string{filepath.Join(xdgConfigHome(home), "nushell", "config.nu")}
	case shell.PowerShell:
		var paths []string
		for _, c := range powershellCandidates(home) {
			paths = append(paths, c.Path)
		}
		return paths
	case shell.Cmd:
		// cmd.exe has no portable profile; users should use PowerShell or eps exec.
		return nil
	default:
		return nil
	}
}

func zshrcPath(home string) string {
	if z := strings.TrimSpace(os.Getenv("ZDOTDIR")); z != "" {
		return filepath.Join(z, ".zshrc")
	}
	return filepath.Join(home, ".zshrc")
}

func xdgConfigHome(home string) string {
	if x := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); x != "" {
		return x
	}
	return filepath.Join(home, ".config")
}

func powershellCandidates(home string) []candidate {
	var out []candidate
	switch runtime.GOOS {
	case "windows":
		docs := windowsDocumentsDir(home)
		out = append(out,
			candidate{shell.PowerShell, filepath.Join(docs, "PowerShell", "Microsoft.PowerShell_profile.ps1")},
			candidate{shell.PowerShell, filepath.Join(docs, "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")},
		)
	default:
		// pwsh on macOS/Linux
		out = append(out,
			candidate{shell.PowerShell, filepath.Join(xdgConfigHome(home), "powershell", "Microsoft.PowerShell_profile.ps1")},
		)
	}
	return out
}

func windowsDocumentsDir(home string) string {
	// Prefer the redirected Documents Known Folder (OneDrive / Group Policy).
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetFolderPath('MyDocuments')").Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p
		}
	}
	return filepath.Join(home, "Documents")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func dedupeTargets(in []Target) []Target {
	seen := map[string]bool{}
	var out []Target
	for _, t := range in {
		if t.Path == "" || seen[t.Path] {
			continue
		}
		// skip cmd — no hook file
		if t.Shell == shell.Cmd {
			continue
		}
		seen[t.Path] = true
		out = append(out, t)
	}
	return out
}

func ensureConfig() (path string, created bool, err error) {
	path, err = config.Path()
	if err != nil {
		return "", false, err
	}
	if fileExists(path) {
		return path, false, nil
	}
	if err := config.Save(config.Default()); err != nil {
		return "", false, err
	}
	return path, true, nil
}

// ExplainDetection returns a human/agent readable summary of discovery policy.
func ExplainDetection() string {
	return fmt.Sprintf(`shell detection policy (self-implemented, no third-party shell libs):
  1. login shell = $SHELL, else getent/dscl account lookup (current: %q)
  2. existing rc/profile files among: .zshrc .bashrc .bash_profile .profile fish config.nu powershell profile
  3. PowerShell profiles: always on Windows; on Unix only if profile already exists
  4. override with: eps setup --shell zsh --shell bash
  5. skip rc edits with: eps setup --no-modify-rc
  6. regenerate hook files only: eps setup --update-only
  note: curl|bash does NOT mean your login shell is bash — we ignore the installer interpreter
  hooks live under ~/.easy-proxy-switch/{sh,fish,powershell,nu}/; rc files get a one-line source
`, loginShell())
}
