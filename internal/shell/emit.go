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
  # Parse like pflag: value shorts consume the rest (-pj => profile=j), last --json wins.
  __ezp_should_passthrough() {
    local json=0 help=0
    local -a args=("$@")
    local i=1 a rest c
    while [ "$i" -lt "${#args[@]}" ]; do
      a="${args[$i]}"
      case "$a" in
        --help|-h|--version) help=1; break ;;
        --) break ;;
        --profile|--http|--https|--socks|--no-proxy|--host|--port|--mode|--node-ca|--shell|-p)
          i=$((i + 2)); continue ;;
        --profile=*|--http=*|--https=*|--socks=*|--no-proxy=*|--host=*|--port=*|--mode=*|--node-ca=*|--shell=*)
          i=$((i + 1)); continue ;;
        --json|-j) json=1; i=$((i + 1)); continue ;;
        --json=false|--json=FALSE|--json=False|--json=0|--json=f|--json=F|-j=false|-j=FALSE|-j=False|-j=0|-j=f|-j=F)
          json=0; i=$((i + 1)); continue ;;
        --json=*|-j=*) json=1; i=$((i + 1)); continue ;;
        --node|--no-node|--uppercase|--no-uppercase|--quiet|-q|--emit)
          i=$((i + 1)); continue ;;
        --node=*|--uppercase=*|--quiet=*|--no-node=*|--no-uppercase=*|--emit=*)
          i=$((i + 1)); continue ;;
        --*) i=$((i + 1)); continue ;;
        -*)
          rest="${a#-}"
          while [ -n "$rest" ]; do
            c="$(printf '%%s' "$rest" | cut -c1)"
            rest="$(printf '%%s' "$rest" | cut -c2-)"
            case "$c" in
              p)
                if [ -n "$rest" ]; then rest=""; else i=$((i + 1)); fi
                ;;
              j) json=1 ;;
              q|h) ;;
              *) ;;
            esac
          done
          i=$((i + 1)); continue ;;
        *) break ;;
      esac
    done
    [ "$help" -eq 1 ] && return 0
    [ "$json" -eq 1 ] && return 0
    return 1
  }
  if [ "$cmd" = "on" ] || [ "$cmd" = "off" ]; then
    if __ezp_should_passthrough "$@"; then
      "$__ezp_bin" "$@"
      return $?
    fi
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
function __ezp_should_passthrough
  set -l json 0
  set -l help 0
  set -l i 2
  while test $i -le (count $argv)
    set -l a $argv[$i]
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
      case --json=false --json=FALSE --json=False --json=0 --json=f --json=F -j=false -j=FALSE -j=False -j=0 -j=f -j=F
        set json 0
        set i (math $i + 1)
        continue
      case "--json=*" "-j=*"
        set json 1
        set i (math $i + 1)
        continue
      case --node --no-node --uppercase --no-uppercase --quiet -q --emit
        set i (math $i + 1)
        continue
      case "--node=*" "--uppercase=*" "--quiet=*" "--no-node=*" "--no-uppercase=*" "--emit=*"
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
              set json 1
            case q h
              true
          end
        end
        set i (math $i + 1)
        continue
      case "*"
        break
    end
  end
  test $help -eq 1; and return 0
  test $json -eq 1; and return 0
  return 1
end

function ezp
  set -l __ezp_bin %s
  set -l cmd
  if set -q argv[1]
    set cmd $argv[1]
  end
  if test "$cmd" = "on" -o "$cmd" = "off"
    if __ezp_should_passthrough $argv
      $__ezp_bin $argv
      return $status
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

  function Test-EzpBoolTrue([string]$v) {
    return $v -match '^(?i:true|1|t)$'
  }
  function Test-EzpBoolFalse([string]$v) {
    return $v -match '^(?i:false|0|f)$'
  }

  function Get-EzpOnOffMode([string[]]$inArgs) {
    # Returns 'passthrough' when final --json is true or help/version; else 'emit'.
    $json = $false
    $valueFlags = @('--profile','-p','--http','--https','--socks','--no-proxy','--host','--port','--mode','--node-ca','--shell')
    $boolFlags = @('--node','--no-node','--uppercase','--no-uppercase','--quiet','-q','--emit')
    $i = 1
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if ($a -in @('--help','-h','--version')) { return 'passthrough' }
      if ($a -eq '--') { break }
      $matched = $false
      foreach ($f in $valueFlags) {
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
      if ($boolFlags -contains $a) { $i++; continue }
      if ($a -match '^--(node|uppercase|quiet|no-node|no-uppercase|emit)=') { $i++; continue }
      if ($a -match '^--') { $i++; continue }
      if ($a -match '^-([^-].*)$') {
        $rest = $Matches[1]
        while ($rest.Length -gt 0) {
          $c = $rest.Substring(0,1)
          $rest = $rest.Substring(1)
          switch ($c) {
            'p' { if ($rest.Length -gt 0) { $rest = '' } else { $i++ } }
            'j' { $json = $true }
            default { }
          }
        }
        $i++; continue
      }
      break
    }
    if ($json) { return 'passthrough' }
    return 'emit'
  }

  function Get-EzpExecForward([string[]]$inArgs) {
    # Accept global flags before exec, then exec flags, then re-insert "--".
    $valueFlags = @('--profile','-p','--http','--https','--socks','--no-proxy','--host','--port','--mode','--node-ca','--shell')
    $boolFlags = @('--node','--no-node','--uppercase','--no-uppercase','--json','-j','--quiet','-q','--help','-h','--version','--emit')
    $execIdx = -1
    $i = 0
    while ($i -lt $inArgs.Count) {
      $a = $inArgs[$i]
      if ($a -eq 'exec') { $execIdx = $i; break }
      $matched = $false
      foreach ($f in @('--shell')) {
        if ($a -eq $f) { $i += 2; $matched = $true; break }
        if ($a.StartsWith("$f=")) { $i++; $matched = $true; break }
      }
      if ($matched) { continue }
      if ($a -in @('--json','-j','--quiet','-q','--help','-h','--version') -or $a -like '--json=*' -or $a -like '-j=*' -or $a -match '^-([jqh]+)$') {
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
      if ($a -match '^--(node|uppercase|quiet|json|no-node|no-uppercase|emit)=') { $i++; continue }
      if ($a -like '-j=*' -or $a -like '--json=*') { $i++; continue }
      if ($a -match '^-([^-].*)$') {
        $rest = $Matches[1]
        $advance = 1
        while ($rest.Length -gt 0) {
          $c = $rest.Substring(0,1)
          $rest = $rest.Substring(1)
          if ($c -eq 'p') {
            if ($rest.Length -eq 0) { $advance = 2 }
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

  if ($Args.Count -ge 1 -and ($Args[0] -eq 'on' -or $Args[0] -eq 'off')) {
    if ((Get-EzpOnOffMode $Args) -eq 'passthrough') {
      & $bin @Args
      if ($LASTEXITCODE -ne 0) { throw "ezp failed with exit $LASTEXITCODE" }
      return
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
  let cmd = ($args | get 0? | default "")

  def is_false_bool [v: string] {
    $v in ["false" "FALSE" "False" "0" "f" "F"]
  }
  def should_passthrough [argv: list<string>] {
    mut json = false
    mut i = 1
    while $i < ($argv | length) {
      let a = ($argv | get $i)
      if $a in ["--help" "-h" "--version"] { return true }
      if $a == "--" { break }
      if $a in ["--profile" "--http" "--https" "--socks" "--no-proxy" "--host" "--port" "--mode" "--node-ca" "--shell" "-p"] {
        $i = $i + 2
        continue
      }
      if ($a | str starts-with "--profile=") or ($a | str starts-with "--http=") or ($a | str starts-with "--https=") or ($a | str starts-with "--socks=") or ($a | str starts-with "--no-proxy=") or ($a | str starts-with "--host=") or ($a | str starts-with "--port=") or ($a | str starts-with "--mode=") or ($a | str starts-with "--node-ca=") or ($a | str starts-with "--shell=") {
        $i = $i + 1
        continue
      }
      if $a in ["--json" "-j"] {
        $json = true
        $i = $i + 1
        continue
      }
      if ($a | str starts-with "--json=") or ($a | str starts-with "-j=") {
        let v = ($a | split row "=" | get 1)
        $json = (not (is_false_bool $v))
        $i = $i + 1
        continue
      }
      if $a in ["--node" "--no-node" "--uppercase" "--no-uppercase" "--quiet" "-q" "--emit"] {
        $i = $i + 1
        continue
      }
      if ($a | str starts-with "--node=") or ($a | str starts-with "--uppercase=") or ($a | str starts-with "--quiet=") or ($a | str starts-with "--emit=") {
        $i = $i + 1
        continue
      }
      if ($a | str starts-with "--") {
        $i = $i + 1
        continue
      }
      if ($a | str starts-with "-") and (not ($a | str starts-with "--")) {
        mut rest = ($a | str substring 1..)
        while ($rest | str length) > 0 {
          let c = ($rest | str substring 0..<1)
          $rest = ($rest | str substring 1..)
          if $c == "p" {
            if ($rest | str length) > 0 {
              $rest = ""
            } else {
              $i = $i + 1
            }
          } else if $c == "j" {
            $json = true
          }
        }
        $i = $i + 1
        continue
      }
      break
    }
    $json
  }
  def strip_json_flags [argv: list<string>] {
    $argv | where {|a|
      not ($a in ["--json" "-j"] or ($a | str starts-with "--json=") or ($a | str starts-with "-j="))
    }
  }

  if $cmd == "on" or $cmd == "off" {
    if (should_passthrough $args) {
      return (^$bin ...$args)
    }
    let rest = (strip_json_flags ($args | skip 1))
    let result = (^$bin $cmd ...$rest --json | complete)
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
