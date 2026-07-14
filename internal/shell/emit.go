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
			// fish `set -e` returns 4 when the var is missing; ignore so off stays success.
			fmt.Fprintf(&b, "set -e %s; or true\n", k)
		case PowerShell:
			fmt.Fprintf(&b, "Remove-Item Env:%s -ErrorAction SilentlyContinue\n", k)
		case Cmd:
			fmt.Fprintf(&b, "set %s=\n", k)
		default:
			fmt.Fprintf(&b, "unset %s\n", k)
		}
	}
	if kind == Fish && len(keys) > 0 {
		b.WriteString("true\n")
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
		// Pure POSIX: top-level helpers + 1-based positional indexing (bash/zsh/dash).
		return fmt.Sprintf(`# easy-proxy-cli shell hook
__ezp_bool_false() {
  case "$1" in false|FALSE|False|0|f|F) return 0 ;; esac
  return 1
}
# Sets: __ezp_cmd_idx __ezp_cmd __ezp_json __ezp_help __ezp_mode(passthrough|emit|forward)
__ezp_scan() {
  __ezp_cmd_idx=0
  __ezp_cmd=
  __ezp_json=0
  __ezp_help=0
  __ezp_mode=forward
  __ezp_n=$#
  __ezp_i=1
  __ezp_seen_cmd=0
  while [ "$__ezp_i" -le "$__ezp_n" ]; do
    eval "__ezp_a=\${$__ezp_i}"
    if [ "$__ezp_seen_cmd" -eq 0 ]; then
      case "$__ezp_a" in
        --help|-h|--version) __ezp_help=1; __ezp_i=$((__ezp_i + 1)); continue ;;
        --) break ;;
        --shell) __ezp_i=$((__ezp_i + 2)); continue ;;
        --shell=*) __ezp_i=$((__ezp_i + 1)); continue ;;
        --json|-j) __ezp_json=1; __ezp_i=$((__ezp_i + 1)); continue ;;
        --quiet|-q) __ezp_i=$((__ezp_i + 1)); continue ;;
        --json=*|-j=*)
          __ezp_v="${__ezp_a#*=}"
          if __ezp_bool_false "$__ezp_v"; then __ezp_json=0; else __ezp_json=1; fi
          __ezp_i=$((__ezp_i + 1)); continue ;;
        --quiet=*|-q=*) __ezp_i=$((__ezp_i + 1)); continue ;;
        -*)
          __ezp_rest="${__ezp_a#-}"
          __ezp_ok=1
          while [ -n "$__ezp_rest" ]; do
            __ezp_c="$(printf '%%s' "$__ezp_rest" | cut -c1)"
            __ezp_rest="$(printf '%%s' "$__ezp_rest" | cut -c2-)"
            case "$__ezp_c" in
              q)
                case "$__ezp_rest" in [=]*) __ezp_rest= ;; esac
                ;;
              j)
                case "$__ezp_rest" in
                  [=]*)
                    __ezp_v="${__ezp_rest#=}"; __ezp_rest=
                    if __ezp_bool_false "$__ezp_v"; then __ezp_json=0; else __ezp_json=1; fi
                    ;;
                  *) __ezp_json=1 ;;
                esac
                ;;
              h) __ezp_help=1 ;;
              *) __ezp_ok=0; break ;;
            esac
          done
          if [ "$__ezp_ok" -eq 1 ]; then __ezp_i=$((__ezp_i + 1)); continue; fi
          break
          ;;
        on|off)
          __ezp_seen_cmd=1
          __ezp_cmd="$__ezp_a"
          __ezp_cmd_idx=$__ezp_i
          __ezp_i=$((__ezp_i + 1))
          continue
          ;;
        *) break ;;
      esac
    else
      case "$__ezp_a" in
        --help|-h|--version) __ezp_help=1; break ;;
        --) break ;;
        --profile|--http|--https|--socks|--no-proxy|--host|--port|--mode|--node-ca|--shell|-p)
          __ezp_i=$((__ezp_i + 2)); continue ;;
        --profile=*|--http=*|--https=*|--socks=*|--no-proxy=*|--host=*|--port=*|--mode=*|--node-ca=*|--shell=*)
          __ezp_i=$((__ezp_i + 1)); continue ;;
        --json|-j) __ezp_json=1; __ezp_i=$((__ezp_i + 1)); continue ;;
        --json=*|-j=*)
          __ezp_v="${__ezp_a#*=}"
          if __ezp_bool_false "$__ezp_v"; then __ezp_json=0; else __ezp_json=1; fi
          __ezp_i=$((__ezp_i + 1)); continue ;;
        --node|--no-node|--uppercase|--no-uppercase|--quiet|-q|--emit)
          __ezp_i=$((__ezp_i + 1)); continue ;;
        --node=*|--uppercase=*|--quiet=*|--no-node=*|--no-uppercase=*|--emit=*|-q=*)
          __ezp_i=$((__ezp_i + 1)); continue ;;
        --*) __ezp_i=$((__ezp_i + 1)); continue ;;
        -*)
          __ezp_rest="${__ezp_a#-}"
          while [ -n "$__ezp_rest" ]; do
            __ezp_c="$(printf '%%s' "$__ezp_rest" | cut -c1)"
            __ezp_rest="$(printf '%%s' "$__ezp_rest" | cut -c2-)"
            case "$__ezp_c" in
              p)
                if [ -n "$__ezp_rest" ]; then __ezp_rest=; else __ezp_i=$((__ezp_i + 1)); fi
                ;;
              j)
                case "$__ezp_rest" in
                  [=]*)
                    __ezp_v="${__ezp_rest#=}"; __ezp_rest=
                    if __ezp_bool_false "$__ezp_v"; then __ezp_json=0; else __ezp_json=1; fi
                    ;;
                  *) __ezp_json=1 ;;
                esac
                ;;
              q)
                case "$__ezp_rest" in [=]*) __ezp_rest= ;; esac
                ;;
              h) __ezp_help=1 ;;
              *) ;;
            esac
          done
          __ezp_i=$((__ezp_i + 1)); continue ;;
        *) break ;;
      esac
    fi
  done
  if [ "$__ezp_cmd_idx" -eq 0 ]; then
    __ezp_mode=forward
    return 0
  fi
  if [ "$__ezp_help" -eq 1 ] || [ "$__ezp_json" -eq 1 ]; then
    __ezp_mode=passthrough
  else
    __ezp_mode=emit
  fi
}
ezp() {
  __ezp_bin=%s
  __ezp_scan "$@"
  if [ "$__ezp_mode" = "forward" ] || [ "$__ezp_mode" = "passthrough" ]; then
    "$__ezp_bin" "$@"
    return $?
  fi
  __ezp_save_n=$#
  __ezp_i=1
  while [ "$__ezp_i" -le "$__ezp_save_n" ]; do
    eval "__ezp_save_$__ezp_i=\${$__ezp_i}"
    __ezp_i=$((__ezp_i + 1))
  done
  __ezp_out=$(
    set --
    __ezp_i=1
    while [ "$__ezp_i" -le "$__ezp_save_n" ]; do
      eval "__ezp_a=\$__ezp_save_$__ezp_i"
      if [ "$__ezp_i" -eq "$__ezp_cmd_idx" ]; then
        set -- "$@" "$__ezp_a" --emit
      else
        set -- "$@" "$__ezp_a"
      fi
      __ezp_i=$((__ezp_i + 1))
    done
    "$__ezp_bin" "$@"
  ) || return $?
  eval "$__ezp_out"
  return $?
}
`, shQuote(binPath)), nil
	case Fish:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (fish)
function __ezp_bool_false
  contains -- $argv[1] false FALSE False 0 f F
end

function __ezp_scan
  set -g __ezp_cmd_idx 0
  set -g __ezp_cmd ""
  set -g __ezp_mode forward
  set -l json 0
  set -l help 0
  set -l i 1
  set -l seen_cmd 0
  set -l n (count $argv)
  while test $i -le $n
    set -l a $argv[$i]
    if test $seen_cmd -eq 0
      switch $a
        case --help -h --version
          set help 1
          set i (math $i + 1)
          continue
        case --
          break
        case --shell
          set i (math $i + 2)
          continue
        case "--shell=*"
          set i (math $i + 1)
          continue
        case --json -j
          set json 1
          set i (math $i + 1)
          continue
        case --quiet -q
          set i (math $i + 1)
          continue
        case "--json=*" "-j=*"
          set -l v (string split -m1 = -- $a)[2]
          if __ezp_bool_false $v
            set json 0
          else
            set json 1
          end
          set i (math $i + 1)
          continue
        case "--quiet=*" "-q=*"
          set i (math $i + 1)
          continue
        case "-*"
          set -l rest (string sub -s 2 -- $a)
          set -l ok 1
          while test -n "$rest"
            set -l c (string sub -l 1 -- $rest)
            set rest (string sub -s 2 -- $rest)
            switch $c
              case q
                if string match -q '=*' -- $rest
                  set rest ""
                end
              case j
                if string match -q '=*' -- $rest
                  set -l v (string sub -s 2 -- $rest)
                  set rest ""
                  if __ezp_bool_false $v
                    set json 0
                  else
                    set json 1
                  end
                else
                  set json 1
                end
              case h
                set help 1
              case '*'
                set ok 0
                break
            end
          end
          if test $ok -eq 1
            set i (math $i + 1)
            continue
          end
          break
        case on off
          set seen_cmd 1
          set __ezp_cmd $a
          set __ezp_cmd_idx $i
          set i (math $i + 1)
          continue
        case '*'
          break
      end
    else
      switch $a
        case --help -h --version
          set help 1
          break
        case --
          break
        case --profile --http --https --socks --no-proxy --host --port --mode --node-ca --shell -p
          set i (math $i + 2)
          continue
        case "--profile=*" "--http=*" "--https=*" "--socks=*" "--no-proxy=*" "--host=*" "--port=*" "--mode=*" "--node-ca=*" "--shell=*"
          set i (math $i + 1)
          continue
        case --json -j
          set json 1
          set i (math $i + 1)
          continue
        case "--json=*" "-j=*"
          set -l v (string split -m1 = -- $a)[2]
          if __ezp_bool_false $v
            set json 0
          else
            set json 1
          end
          set i (math $i + 1)
          continue
        case --node --no-node --uppercase --no-uppercase --quiet -q --emit
          set i (math $i + 1)
          continue
        case "--node=*" "--uppercase=*" "--quiet=*" "--no-node=*" "--no-uppercase=*" "--emit=*" "-q=*"
          set i (math $i + 1)
          continue
        case "--*"
          set i (math $i + 1)
          continue
        case "-*"
          set -l rest (string sub -s 2 -- $a)
          while test -n "$rest"
            set -l c (string sub -l 1 -- $rest)
            set rest (string sub -s 2 -- $rest)
            switch $c
              case p
                if test -n "$rest"
                  set rest ""
                else
                  set i (math $i + 1)
                end
              case j
                if string match -q '=*' -- $rest
                  set -l v (string sub -s 2 -- $rest)
                  set rest ""
                  if __ezp_bool_false $v
                    set json 0
                  else
                    set json 1
                  end
                else
                  set json 1
                end
              case q
                if string match -q '=*' -- $rest
                  set rest ""
                end
              case h
                set help 1
            end
          end
          set i (math $i + 1)
          continue
        case '*'
          break
      end
    end
  end
  if test $__ezp_cmd_idx -eq 0
    set __ezp_mode forward
    return
  end
  if test $help -eq 1 -o $json -eq 1
    set __ezp_mode passthrough
  else
    set __ezp_mode emit
  end
end

function ezp
  set -l __ezp_bin %s
  __ezp_scan $argv
  if test "$__ezp_mode" = forward -o "$__ezp_mode" = passthrough
    $__ezp_bin $argv
    return $status
  end
  set -l forward
  set -l i 1
  for a in $argv
    if test $i -eq $__ezp_cmd_idx
      set forward $forward $a --emit --shell fish
    else
      set forward $forward $a
    end
    set i (math $i + 1)
  end
  set -l __ezp_out ($__ezp_bin $forward | string collect)
  set -l __ezp_status $pipestatus[1]
  if test $__ezp_status -ne 0
    return $__ezp_status
  end
  eval $__ezp_out
  return $status
end
`, fishQuote(binPath)), nil
	case PowerShell:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (PowerShell)
function ezp {
  param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Args)
  $bin = %s
  if ($null -eq $Args) { $Args = @() }

  function Test-EzpBoolFalse([string]$v) {
    return $v -match '^(?i:false|0|f)$'
  }

  function Get-EzpScan([string[]]$inArgs) {
    $json = $false
    $help = $false
    $cmdIdx = -1
    $cmd = $null
    $i = 0
    $seenCmd = $false
    $valueAfterCmd = @('--profile','-p','--http','--https','--socks','--no-proxy','--host','--port','--mode','--node-ca','--shell')
    $boolAfterCmd = @('--node','--no-node','--uppercase','--no-uppercase','--quiet','-q','--emit')
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if (-not $seenCmd) {
        if ($a -in @('--help','-h','--version')) { $help = $true; $i++; continue }
        if ($a -eq '--') { break }
        if ($a -eq '--shell') { $i += 2; continue }
        if ($a.StartsWith('--shell=')) { $i++; continue }
        if ($a -eq '--json' -or $a -eq '-j') { $json = $true; $i++; continue }
        if ($a -eq '--quiet' -or $a -eq '-q') { $i++; continue }
        if ($a -like '--json=*' -or $a -like '-j=*') {
          $v = $a.Substring($a.IndexOf('=') + 1)
          if (Test-EzpBoolFalse $v) { $json = $false } else { $json = $true }
          $i++; continue
        }
        if ($a -like '--quiet=*' -or $a -like '-q=*') { $i++; continue }
        if ($a -match '^-([^-].*)$') {
          $rest = $Matches[1]
          $ok = $true
          while ($rest.Length -gt 0) {
            $c = $rest.Substring(0,1)
            $rest = $rest.Substring(1)
            switch ($c) {
              'q' { if ($rest.StartsWith('=')) { $rest = '' } }
              'j' {
                if ($rest.StartsWith('=')) {
                  $v = $rest.Substring(1); $rest = ''
                  if (Test-EzpBoolFalse $v) { $json = $false } else { $json = $true }
                } else { $json = $true }
              }
              'h' { $help = $true }
              default { $ok = $false }
            }
            if (-not $ok) { break }
          }
          if ($ok) { $i++; continue }
          break
        }
        if ($a -eq 'on' -or $a -eq 'off') {
          $seenCmd = $true; $cmd = $a; $cmdIdx = $i; $i++; continue
        }
        break
      }
      if ($a -in @('--help','-h','--version')) { $help = $true; break }
      if ($a -eq '--') { break }
      $matched = $false
      foreach ($f in $valueAfterCmd) {
        if ($a -eq $f) { $i += 2; $matched = $true; break }
        if ($a.StartsWith("$f=")) { $i++; $matched = $true; break }
      }
      if ($matched) { continue }
      if ($a -eq '--json' -or $a -eq '-j') { $json = $true; $i++; continue }
      if ($a -like '--json=*' -or $a -like '-j=*') {
        $v = $a.Substring($a.IndexOf('=') + 1)
        if (Test-EzpBoolFalse $v) { $json = $false } else { $json = $true }
        $i++; continue
      }
      if ($boolAfterCmd -contains $a) { $i++; continue }
      if ($a -match '^--(node|uppercase|quiet|no-node|no-uppercase|emit)=' -or $a -like '-q=*') { $i++; continue }
      if ($a -match '^--') { $i++; continue }
      if ($a -match '^-([^-].*)$') {
        $rest = $Matches[1]
        while ($rest.Length -gt 0) {
          $c = $rest.Substring(0,1)
          $rest = $rest.Substring(1)
          switch ($c) {
            'p' { if ($rest.Length -gt 0) { $rest = '' } else { $i++ } }
            'j' {
              if ($rest.StartsWith('=')) {
                $v = $rest.Substring(1); $rest = ''
                if (Test-EzpBoolFalse $v) { $json = $false } else { $json = $true }
              } else { $json = $true }
            }
            'q' { if ($rest.StartsWith('=')) { $rest = '' } }
            'h' { $help = $true }
            default { }
          }
        }
        $i++; continue
      }
      break
    }
    $mode = 'forward'
    if ($cmdIdx -ge 0) {
      if ($help -or $json) { $mode = 'passthrough' } else { $mode = 'emit' }
    }
    return @{ Mode = $mode; CmdIdx = $cmdIdx; Cmd = $cmd }
  }

  function Get-EzpExecForward([string[]]$inArgs) {
    $valueFlags = @('--profile','-p','--http','--https','--socks','--no-proxy','--host','--port','--mode','--node-ca','--shell')
    $boolFlags = @('--node','--no-node','--uppercase','--no-uppercase','--json','-j','--quiet','-q','--help','-h','--version','--emit')
    $execIdx = -1
    $i = 0
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if ($a -eq 'exec') { $execIdx = $i; break }
      if ($a -eq '--shell') { $i += 2; continue }
      if ($a.StartsWith('--shell=')) { $i++; continue }
      if ($a -in @('--json','-j','--quiet','-q','--help','-h','--version') -or $a -like '--json=*' -or $a -like '-j=*' -or $a -like '--quiet=*' -or $a -like '-q=*' -or $a -match '^-([jqh]+)$' -or $a -match '^-([jqh]+)=') {
        $i++; continue
      }
      break
    }
    if ($execIdx -lt 0) { return ,$inArgs }
    $globals = @()
    if ($execIdx -gt 0) { $globals = $inArgs[0..($execIdx-1)] }
    $i = $execIdx + 1
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if ($a -eq '--') { $i++; break }
      $matched = $false
      foreach ($f in $valueFlags) {
        if ($a -eq $f) { $i += 2; $matched = $true; break }
        if ($a.StartsWith("$f=")) { $i++; $matched = $true; break }
      }
      if ($matched) { continue }
      if ($boolFlags -contains $a) { $i++; continue }
      if ($a -match '^--(node|uppercase|quiet|json|no-node|no-uppercase|emit)=' -or $a -like '-j=*' -or $a -like '--json=*' -or $a -like '-q=*') { $i++; continue }
      if ($a -match '^-([^-].*)$') {
        $rest = $Matches[1]
        $advance = 1
        while ($rest.Length -gt 0) {
          $c = $rest.Substring(0,1)
          $rest = $rest.Substring(1)
          if ($c -eq 'p') {
            if ($rest.Length -eq 0) { $advance = 2 }
            $rest = ''
          } elseif ($rest.StartsWith('=')) {
            $rest = ''
          }
        }
        $i += $advance
        continue
      }
      break
    }
    $forward = New-Object System.Collections.Generic.List[string]
    foreach ($x in $globals) { [void]$forward.Add($x) }
    [void]$forward.Add('exec')
    $flagEnd = $i
    $flagStart = $execIdx + 1
    if ($flagEnd -gt $flagStart) {
      foreach ($x in $inArgs[$flagStart..($flagEnd-1)]) { [void]$forward.Add($x) }
    }
    [void]$forward.Add('--')
    if ($i -lt $inArgs.Count) {
      foreach ($x in $inArgs[$i..($inArgs.Count-1)]) { [void]$forward.Add($x) }
    }
    return ,$forward.ToArray()
  }

  $scan = Get-EzpScan $Args
  if ($scan.Mode -eq 'passthrough') {
    & $bin @Args
    if ($LASTEXITCODE -ne 0) { throw "ezp failed with exit $LASTEXITCODE" }
    return
  }
  if ($scan.Mode -eq 'emit') {
    $forward = New-Object System.Collections.Generic.List[string]
    for ($i = 0; $i -lt $Args.Count; $i++) {
      [void]$forward.Add($Args[$i])
      if ($i -eq $scan.CmdIdx) {
        [void]$forward.Add('--emit')
        [void]$forward.Add('--shell')
        [void]$forward.Add('powershell')
      }
    }
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $raw = & $bin @($forward.ToArray()) 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    $text = ($raw | ForEach-Object { "$_" }) -join [Environment]::NewLine
    if ($code -ne 0) {
      if ($text) { Write-Error $text }
      throw "ezp $($scan.Cmd) failed with exit $code"
    }
    if ($text) { Invoke-Expression $text }
    return
  }
  if ($Args -contains 'exec') {
    $forward = Get-EzpExecForward $Args
    & $bin @forward
    return
  }
  & $bin @Args
}
`, psQuote(binPath)), nil
	case Nu:
		return fmt.Sprintf(`# easy-proxy-cli shell hook (nushell)
def --env --wrapped ezp [...args: string] {
  let bin = %s

  def is_false_bool [v: string] {
    $v in ["false" "FALSE" "False" "0" "f" "F"]
  }
  def scan [argv: list<string>] {
    mut json = false
    mut help = false
    mut cmd_idx = -1
    mut cmd = ""
    mut i = 0
    mut seen_cmd = false
    while $i < ($argv | length) {
      let a = ($argv | get $i)
      if not $seen_cmd {
        if $a in ["--help" "-h" "--version"] { $help = true; $i = $i + 1; continue }
        if $a == "--" { break }
        if $a == "--shell" { $i = $i + 2; continue }
        if ($a | str starts-with "--shell=") { $i = $i + 1; continue }
        if $a in ["--json" "-j"] { $json = true; $i = $i + 1; continue }
        if $a in ["--quiet" "-q"] { $i = $i + 1; continue }
        if ($a | str starts-with "--json=") or ($a | str starts-with "-j=") {
          let v = ($a | split row "=" | get 1)
          $json = (not (is_false_bool $v))
          $i = $i + 1
          continue
        }
        if ($a | str starts-with "--quiet=") or ($a | str starts-with "-q=") { $i = $i + 1; continue }
        if ($a | str starts-with "-") and (not ($a | str starts-with "--")) {
          mut rest = ($a | str substring 1..)
          mut ok = true
          while ($rest | str length) > 0 {
            let c = ($rest | str substring 0..<1)
            $rest = ($rest | str substring 1..)
            if $c == "q" {
              if ($rest | str starts-with "=") { $rest = "" }
            } else if $c == "j" {
              if ($rest | str starts-with "=") {
                let v = ($rest | str substring 1..)
                $rest = ""
                $json = (not (is_false_bool $v))
              } else { $json = true }
            } else if $c == "h" {
              $help = true
            } else { $ok = false; break }
          }
          if $ok { $i = $i + 1; continue }
          break
        }
        if $a in ["on" "off"] {
          $seen_cmd = true
          $cmd = $a
          $cmd_idx = $i
          $i = $i + 1
          continue
        }
        break
      } else {
        if $a in ["--help" "-h" "--version"] { $help = true; break }
        if $a == "--" { break }
        if $a in ["--profile" "--http" "--https" "--socks" "--no-proxy" "--host" "--port" "--mode" "--node-ca" "--shell" "-p"] {
          $i = $i + 2; continue
        }
        if ($a | str starts-with "--profile=") or ($a | str starts-with "--http=") or ($a | str starts-with "--https=") or ($a | str starts-with "--socks=") or ($a | str starts-with "--no-proxy=") or ($a | str starts-with "--host=") or ($a | str starts-with "--port=") or ($a | str starts-with "--mode=") or ($a | str starts-with "--node-ca=") or ($a | str starts-with "--shell=") {
          $i = $i + 1; continue
        }
        if $a in ["--json" "-j"] { $json = true; $i = $i + 1; continue }
        if ($a | str starts-with "--json=") or ($a | str starts-with "-j=") {
          let v = ($a | split row "=" | get 1)
          $json = (not (is_false_bool $v))
          $i = $i + 1
          continue
        }
        if $a in ["--node" "--no-node" "--uppercase" "--no-uppercase" "--quiet" "-q" "--emit"] {
          $i = $i + 1; continue
        }
        if ($a | str starts-with "--node=") or ($a | str starts-with "--uppercase=") or ($a | str starts-with "--quiet=") or ($a | str starts-with "--emit=") or ($a | str starts-with "-q=") {
          $i = $i + 1; continue
        }
        if ($a | str starts-with "--") { $i = $i + 1; continue }
        if ($a | str starts-with "-") and (not ($a | str starts-with "--")) {
          mut rest = ($a | str substring 1..)
          while ($rest | str length) > 0 {
            let c = ($rest | str substring 0..<1)
            $rest = ($rest | str substring 1..)
            if $c == "p" {
              if ($rest | str length) > 0 { $rest = "" } else { $i = $i + 1 }
            } else if $c == "j" {
              if ($rest | str starts-with "=") {
                let v = ($rest | str substring 1..)
                $rest = ""
                $json = (not (is_false_bool $v))
              } else { $json = true }
            } else if $c == "q" {
              if ($rest | str starts-with "=") { $rest = "" }
            } else if $c == "h" {
              $help = true
            }
          }
          $i = $i + 1
          continue
        }
        break
      }
    }
    let mode = if $cmd_idx < 0 {
      "forward"
    } else if $help or $json {
      "passthrough"
    } else {
      "emit"
    }
    {mode: $mode, cmd_idx: $cmd_idx, cmd: $cmd}
  }
  def entries_to_record [entries: list] {
    # Nushell env is case-insensitive; prefer lowercase proxy keys so curl sees http_proxy.
    let prefer_lower = ["http_proxy" "https_proxy" "all_proxy" "no_proxy"]
    mut rec = {}
    for e in $entries {
      let k = $e.key
      let lk = ($k | str downcase)
      if $lk in $prefer_lower {
        if $k != $lk { continue }
      }
      $rec = ($rec | upsert $k $e.value)
    }
    $rec
  }

  let s = (scan $args)
  if $s.mode == "forward" or $s.mode == "passthrough" {
    return (^$bin ...$args)
  }
  let result = (^$bin ...$args --json | complete)
  if $result.exit_code != 0 {
    error make {msg: $"ezp ($s.cmd) failed", label: {text: ($result.stderr | str trim)}}
  }
  let data = ($result.stdout | from json)
  if $s.cmd == "on" {
    for k in ($data.unset? | default []) {
      hide-env -i $k
    }
    # Drop uppercase aliases before loading lowercase-only records so inherited
    # HTTP_PROXY cannot linger when entries_to_record skips uppercase mirrors.
    for k in ["HTTP_PROXY" "HTTPS_PROXY" "ALL_PROXY" "NO_PROXY"] {
      hide-env -i $k
    }
    load-env (entries_to_record $data.env)
  } else {
    for k in $data.unset {
      hide-env -i $k
    }
  }
  return
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
