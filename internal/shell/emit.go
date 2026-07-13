package shell

import (
	"fmt"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/apperr"
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
		return "", apperr.Misconfigf("unsupported shell %q (bash|zsh|sh|fish|powershell|cmd|nu)", name)
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
			fmt.Fprintf(&b, "set \"%s=%s\"\n", k, cmdEscape(v))
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
  local cmd="${1-}"
  if [ "$cmd" = "on" ] || [ "$cmd" = "off" ]; then
    local __a
    for __a in "$@"; do
      case "$__a" in
        --json|--json=*|-j|-j=*|-j?*|--help|-h|--version)
          "$__ezp_bin" "$@"
          return $?
          ;;
      esac
    done
    shift
    local __ezp_out
    __ezp_out="$("$__ezp_bin" "$cmd" --emit "$@")" || return $?
    eval "$__ezp_out"
    return $?
  fi
  "$__ezp_bin" "$@"
}
`, shQuote(binPath)), nil
	case Fish:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (fish)
function ezp
  set -l __ezp_bin %s
  set -l cmd
  if set -q argv[1]
    set cmd $argv[1]
  end
  if test "$cmd" = "on" -o "$cmd" = "off"
    for __a in $argv
      switch $__a
        case --json "--json=*" -j "-j=*" "-j?*" --help -h --version
          $__ezp_bin $argv
          return $status
      end
    end
    set -e argv[1]
    set -l __ezp_out ($__ezp_bin $cmd --emit --shell fish $argv | string collect)
    set -l __ezp_status $pipestatus[1]
    if test $__ezp_status -ne 0
      return $__ezp_status
    end
    eval $__ezp_out
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
  if ($null -eq $Args) { $Args = @() }
  function Test-EzpReadonlyFlag([string]$a) {
    if ($a -in @('--json','-j','--help','-h','--version')) { return $true }
    if ($a -like '--json=*') { return $true }
    if ($a -like '-j=*') { return $true }
    if ($a -match '^-j.') { return $true }
    return $false
  }
  if ($Args.Count -ge 1 -and ($Args[0] -eq 'on' -or $Args[0] -eq 'off')) {
    foreach ($a in $Args) {
      if (Test-EzpReadonlyFlag $a) {
        & $bin @Args
        if ($LASTEXITCODE -ne 0) { throw "ezp failed with exit $LASTEXITCODE" }
        return
      }
    }
    $cmd = $Args[0]
    $rest = @()
    if ($Args.Count -gt 1) { $rest = $Args[1..($Args.Count-1)] }
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $raw = & $bin $cmd --emit --shell powershell @rest 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    $text = ($raw | ForEach-Object { "$_" }) -join [Environment]::NewLine
    if ($code -ne 0) {
      if ($text) { Write-Error $text }
      throw "ezp $cmd failed with exit $code"
    }
    if ($text) { Invoke-Expression $text }
    return
  }
  if ($Args.Count -ge 1 -and $Args[0] -eq 'exec') {
    # PowerShell consumes a literal "--" during binding; re-insert for Cobra.
    $rest = @()
    if ($Args.Count -gt 1) { $rest = $Args[1..($Args.Count-1)] }
    & $bin exec -- @rest
    return
  }
  & $bin @Args
}
`, psQuote(binPath)), nil
	case Nu:
		// Prefer --json so values with quotes/special chars stay structured.
		return fmt.Sprintf(`# easy-proxy-cli shell hook (nushell)
def --env --wrapped ezp [...args: string] {
  let bin = %s
  let cmd = ($args | get 0? | default "")
  let is_readonly = {|a|
    ($a == "--json") or ($a == "-j") or ($a == "--help") or ($a == "-h") or ($a == "--version") or ($a | str starts-with "--json=") or ($a | str starts-with "-j=") or (($a | str starts-with "-j") and (($a | str length) > 2))
  }
  if $cmd == "on" or $cmd == "off" {
    if ($args | any $is_readonly) {
      return (^$bin ...$args)
    }
    let rest = ($args | skip 1)
    let result = (^$bin $cmd --json ...$rest | complete)
    if $result.exit_code != 0 {
      error make {msg: $"ezp ($cmd) failed", label: {text: ($result.stderr | str trim)}}
    }
    let data = ($result.stdout | from json)
    if $cmd == "on" {
      load-env $data.env
    } else {
      for k in $data.unset {
        hide-env -i $k
      }
    }
    return
  }
  return (^$bin ...$args)
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

func cmdEscape(s string) string {
	// Inside set "KEY=value": escape % (env expansion) and " (string terminator).
	s = strings.ReplaceAll(s, `%`, `%%`)
	s = strings.ReplaceAll(s, `"`, `""`)
	return s
}

func nuQuote(s string) string {
	// Nushell string literal (backticks execute commands).
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
