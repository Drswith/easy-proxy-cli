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
  __ezp_json_true() {
    case "$1" in
      --json|-j) return 0 ;;
      --json=false|--json=FALSE|--json=False|--json=0|--json=f|--json=F) return 1 ;;
      --json=true|--json=TRUE|--json=True|--json=1|--json=t|--json=T) return 0 ;;
      --json=*) return 0 ;;
      -j=false|-j=FALSE|-j=False|-j=0|-j=f|-j=F) return 1 ;;
      -j=true|-j=TRUE|-j=True|-j=1|-j=t|-j=T) return 0 ;;
      -j=*) return 0 ;;
      --help|-h|--version) return 0 ;;
      -*=*) return 1 ;;
      -*)
        case "${1#-}" in
          *j*) return 0 ;;
        esac
        ;;
    esac
    return 1
  }
  if [ "$cmd" = "on" ] || [ "$cmd" = "off" ]; then
    local __a
    for __a in "$@"; do
      if __ezp_json_true "$__a"; then
        "$__ezp_bin" "$@"
        return $?
      fi
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
function __ezp_json_true
  set -l a $argv[1]
  switch $a
    case --json -j --help -h --version
      return 0
    case --json=false --json=FALSE --json=False --json=0 --json=f --json=F -j=false -j=FALSE -j=False -j=0 -j=f -j=F
      return 1
    case "--json=*" "-j=*"
      return 0
    case "-*"
      set -l rest (string sub -s 2 -- $a)
      if string match -q -r '^[^=]*j' -- $rest
        return 0
      end
      return 1
  end
  return 1
end

function ezp
  set -l __ezp_bin %s
  set -l cmd
  if set -q argv[1]
    set cmd $argv[1]
  end
  if test "$cmd" = "on" -o "$cmd" = "off"
    for __a in $argv
      if __ezp_json_true $__a
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
  function Test-EzpJsonTrue([string]$a) {
    switch -Regex ($a) {
      '^(--json|-j|--help|-h|--version)$' { return $true }
      '^--json=(?i:false|0|f)$' { return $false }
      '^-j=(?i:false|0|f)$' { return $false }
      '^--json=' { return $true }
      '^-j=' { return $true }
      '^-([^=]*j[^=]*)$' { return $true }
      default { return $false }
    }
  }
  function Get-EzpExecForward([string[]]$inArgs) {
    # PowerShell drops a literal "--"; re-insert it after exec's own flags.
    $valueFlags = @('--profile','-p','--http','--https','--socks','--no-proxy','--host','--port','--mode','--node-ca','--shell')
    $boolFlags = @('--node','--no-node','--uppercase','--no-uppercase','--json','-j','--quiet','-q','--help','-h','--version','--emit')
    $i = 1
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if ($a -eq '--') { $i++; break }
      $matched = $false
      foreach ($f in $valueFlags) {
        if ($a -eq $f) {
          $i += 2
          $matched = $true
          break
        }
        if ($a.StartsWith("$f=")) {
          $i++
          $matched = $true
          break
        }
      }
      if ($matched) { continue }
      if ($boolFlags -contains $a -or $a -like '--json=*' -or $a -like '-j=*' -or $a -match '^-([jqh]+)$') {
        $i++
        continue
      }
      break
    }
    $forward = New-Object System.Collections.Generic.List[string]
    [void]$forward.Add('exec')
    if ($i -gt 1) {
      foreach ($x in $inArgs[1..($i-1)]) { [void]$forward.Add($x) }
    }
    [void]$forward.Add('--')
    if ($i -lt $inArgs.Count) {
      foreach ($x in $inArgs[$i..($inArgs.Count-1)]) { [void]$forward.Add($x) }
    }
    return ,$forward.ToArray()
  }
  if ($Args.Count -ge 1 -and ($Args[0] -eq 'on' -or $Args[0] -eq 'off')) {
    foreach ($a in $Args) {
      if (Test-EzpJsonTrue $a) {
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
    $forward = Get-EzpExecForward $Args
    & $bin @forward
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
  let is_json_true = {|a|
    if $a in ["--json" "-j" "--help" "-h" "--version"] { true } else if $a in ["--json=false" "--json=FALSE" "--json=False" "--json=0" "--json=f" "--json=F" "-j=false" "-j=FALSE" "-j=False" "-j=0" "-j=f" "-j=F"] { false } else if ($a | str starts-with "--json=") { true } else if ($a | str starts-with "-j=") { true } else if (($a | str starts-with "-") and (not ($a | str contains "=")) and ($a | str contains "j")) { true } else { false }
  }
  if $cmd == "on" or $cmd == "off" {
    if ($args | any $is_json_true) {
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
