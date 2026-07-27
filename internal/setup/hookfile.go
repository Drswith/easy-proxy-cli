package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drswith/easy-proxy-switch-cli/internal/config"
	"github.com/drswith/easy-proxy-switch-cli/internal/shell"
)

const (
	hookFileHeader = `# eps Shell Integration - DO NOT EDIT MANUALLY
# Managed by eps setup. Regenerate with: eps setup --update-only
`
	configRootVar = "${EPS_HOME:-$HOME/.easy-proxy-switch}"
)

// hookRelPath is the managed hook script path relative to the eps config directory.
func hookRelPath(kind shell.Kind) (string, error) {
	switch kind {
	case shell.Zsh, shell.Bash, shell.Posix:
		return "sh/eps.sh", nil
	case shell.Fish:
		return "fish/eps.fish", nil
	case shell.PowerShell:
		return "powershell/eps.ps1", nil
	case shell.Nu:
		return "nu/eps.nu", nil
	default:
		return "", fmt.Errorf("hook not supported for shell %q", kind)
	}
}

func hookFilePath(kind shell.Kind) (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	rel, err := hookRelPath(kind)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, rel), nil
}

func wrapHookFile(script string) string {
	return hookFileHeader + strings.TrimRight(script, "\n") + "\n"
}

// wrapSourceBlock is the thin rc/profile snippet that sources the managed hook file.
func wrapSourceBlock(kind shell.Kind) (string, error) {
	line, err := sourceLine(kind)
	if err != nil {
		return "", err
	}
	return MarkerBegin + "\n" + line + "\n" + MarkerEnd + "\n", nil
}

func sourceLine(kind shell.Kind) (string, error) {
	rel, err := hookRelPath(kind)
	if err != nil {
		return "", err
	}
	hook := configRootVar + "/" + rel
	switch kind {
	case shell.Zsh, shell.Bash:
		return fmt.Sprintf(`[[ -f "%s" ]] && . "%s" # eps Shell Integration`, hook, hook), nil
	case shell.Posix:
		return fmt.Sprintf(`[ -f "%s" ] && . "%s" # eps Shell Integration`, hook, hook), nil
	case shell.Fish:
		return fmt.Sprintf(
			`set -q EPS_HOME; or set -gx EPS_HOME "$HOME/.easy-proxy-switch"; test -f "$EPS_HOME/%s"; and source "$EPS_HOME/%s" # eps Shell Integration`,
			rel, rel,
		), nil
	case shell.PowerShell:
		return fmt.Sprintf(
			`$_epsHome = if ($env:EPS_HOME) { $env:EPS_HOME } else { Join-Path $HOME '.easy-proxy-switch' }; if (Test-Path (Join-Path $_epsHome '%s')) { . (Join-Path $_epsHome '%s') } # eps Shell Integration`,
			rel, rel,
		), nil
	case shell.Nu:
		return fmt.Sprintf(
			`let _eps_home = ($env.EPS_HOME | default ($env.HOME | path join '.easy-proxy-switch')); if ((_eps_home | path join '%s') | path exists) { source ($_eps_home | path join '%s') } # eps Shell Integration`,
			rel, rel,
		), nil
	default:
		return "", fmt.Errorf("source line not supported for shell %q", kind)
	}
}

func writeHookFile(path, content string, dryRun bool) (string, error) {
	existed := fileExists(path)
	if existed {
		data, err := os.ReadFile(path)
		if err != nil {
			return "skipped", err
		}
		if string(data) == content {
			return "skipped", nil
		}
	}
	if dryRun {
		if !existed {
			return "would_write", nil
		}
		return "would_update", nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "skipped", err
	}
	mode := os.FileMode(0o644)
	if existed {
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return "skipped", err
	}
	if !existed {
		return "written", nil
	}
	return "updated", nil
}

func removeHookFile(path string, dryRun bool) (string, error) {
	if !fileExists(path) {
		return "skipped", nil
	}
	if dryRun {
		return "would_remove", nil
	}
	if err := os.Remove(path); err != nil {
		return "skipped", err
	}
	return "removed", nil
}
