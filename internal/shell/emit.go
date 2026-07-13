package shell

import (
	"fmt"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/proxy"
)

// Kind is a supported shell dialect for emit/hook.
type Kind string

const (
	Bash       Kind = "bash"
	Zsh        Kind = "zsh"
	Fish       Kind = "fish"
	PowerShell Kind = "powershell"
	Cmd        Kind = "cmd"
	Posix      Kind = "sh"
	Nu         Kind = "nu"
)

func Detect(name string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bash":
		return Bash, nil
	case "zsh":
		return Zsh, nil
	case "", "sh", "posix":
		return Posix, nil
	case "fish":
		return Fish, nil
	case "powershell", "pwsh", "ps":
		return PowerShell, nil
	case "cmd", "bat":
		return Cmd, nil
	case "nu", "nushell":
		return Nu, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (bash|zsh|sh|fish|powershell|cmd|nu)", name)
	}
}

// EmitExport prints shell statements that set env vars.
func EmitExport(kind Kind, env map[string]string) string {
	var b strings.Builder
	for _, k := range proxy.SortedKeys(env) {
		v := env[k]
		switch kind {
		case Fish:
			fmt.Fprintf(&b, "set -gx %s %s;\n", k, fishQuote(v))
		case PowerShell:
			fmt.Fprintf(&b, "$env:%s = %s\n", k, psQuote(v))
		case Cmd:
			fmt.Fprintf(&b, "set %s=%s\n", k, v)
		default:
			fmt.Fprintf(&b, "export %s=%s\n", k, shQuote(v))
		}
	}
	return b.String()
}

// EmitUnset prints shell statements that clear managed env vars.
func EmitUnset(kind Kind, keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		switch kind {
		case Fish:
			fmt.Fprintf(&b, "set -e %s;\n", k)
		case PowerShell:
			fmt.Fprintf(&b, "Remove-Item Env:%s -ErrorAction SilentlyContinue\n", k)
		case Cmd:
			fmt.Fprintf(&b, "set %s=\n", k)
		default:
			fmt.Fprintf(&b, "unset %s\n", k)
		}
	}
	return b.String()
}

// HookScript installs a shell wrapper so `ezp on|off` can mutate the current shell.
func HookScript(kind Kind, binPath string) (string, error) {
	if binPath == "" {
		binPath = "ezp"
	}
	switch kind {
	case Bash, Zsh, Posix:
		return fmt.Sprintf(`# easy-proxy-cli shell hook
ezp() {
  local __ezp_bin=%s
  local cmd="$1"
  if [ "$cmd" = "on" ] || [ "$cmd" = "off" ]; then
    shift
    eval "$("$__ezp_bin" "$cmd" --emit "$@")"
    return $?
  fi
  "$__ezp_bin" "$@"
}
`, shQuote(binPath)), nil
	case Fish:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (fish)
function ezp
  set -l __ezp_bin %s
  set -l cmd $argv[1]
  if test "$cmd" = "on" -o "$cmd" = "off"
    set -e argv[1]
    eval ($__ezp_bin $cmd --emit $argv | string collect)
    return $status
  end
  $__ezp_bin $argv
end
`, fishQuote(binPath)), nil
	case PowerShell:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (PowerShell)
function ezp {
  param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Args)
  $bin = %s
  if ($Args.Count -ge 1 -and ($Args[0] -eq 'on' -or $Args[0] -eq 'off')) {
    $cmd = $Args[0]
    $rest = @()
    if ($Args.Count -gt 1) { $rest = $Args[1..($Args.Count-1)] }
    Invoke-Expression (& $bin $cmd --emit @rest | Out-String)
    return
  }
  & $bin @Args
}
`, psQuote(binPath)), nil
	case Nu:
		// Nushell: parse POSIX export/unset lines from `ezp on|off --emit`.
		return fmt.Sprintf(`# easy-proxy-cli shell hook (nushell)
def --env --wrapped ezp [...args: string] {
  let bin = %s
  let cmd = ($args | get 0? | default "")
  if $cmd == "on" or $cmd == "off" {
    let rest = ($args | skip 1)
    let script = (^$bin $cmd --emit --shell sh ...$rest)
    mut map = {}
    for line in ($script | lines) {
      let t = ($line | str trim)
      if ($t | str starts-with "export ") {
        let body = ($t | str replace "export " "")
        let eq = ($body | str index-of "=")
        if $eq != null {
          let key = ($body | str substring 0..<$eq)
          mut val = ($body | str substring ($eq + 1)..)
          if ($val | str starts-with "'") and ($val | str ends-with "'") {
            $val = ($val | str substring 1..<($val | str length | $in - 1))
          }
          $map = ($map | upsert $key $val)
        }
      } else if ($t | str starts-with "unset ") {
        let key = ($t | str replace "unset " "" | str trim)
        hide-env -i $key
      }
    }
    if not ($map | is-empty) {
      load-env $map
    }
    return
  }
  ^$bin ...$args
}
`, nuQuote(binPath)), nil
	default:
		return "", fmt.Errorf("hook not supported for shell %q", kind)
	}
}

func shQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func fishQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `\'`) + "'"
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func nuQuote(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
