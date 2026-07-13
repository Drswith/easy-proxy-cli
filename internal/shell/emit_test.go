package shell_test

import (
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-cli/internal/shell"
)

func TestHookScriptPreservesBinaryStatus(t *testing.T) {
	script, err := shell.HookScript(shell.Bash, "/usr/bin/ezp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `|| return $?`) {
		t.Fatalf("hook must return binary status before eval:\n%s", script)
	}
	if !strings.Contains(script, `__ezp_scan`) {
		t.Fatalf("hook must scan globals before on/off:\n%s", script)
	}
	if !strings.Contains(script, `eval "__ezp_a=\${$__ezp_i}"`) && !strings.Contains(script, `eval "__ezp_a=\${$__ezp_i}"`) {
		// 1-based positional access (POSIX-safe for bash/zsh/dash)
		if !strings.Contains(script, `__ezp_i`) {
			t.Fatalf("hook must use positional indexing:\n%s", script)
		}
	}
	if strings.Contains(script, `local -a`) {
		t.Fatalf("shared hook must not use bash arrays:\n%s", script)
	}
	if !strings.Contains(script, `__ezp_bool_false`) {
		t.Fatalf("hook must parse bool assignments:\n%s", script)
	}
	if !strings.Contains(script, `[=]*`) {
		t.Fatalf("hook must use zsh-safe [=]* patterns:\n%s", script)
	}
}

func TestPosixHookIsDashSafe(t *testing.T) {
	script, err := shell.HookScript(shell.Posix, "/usr/bin/ezp")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, `local -a`) || strings.Contains(script, `local `) {
		t.Fatalf("posix hook must avoid local/arrays:\n%s", script)
	}
	zsh, err := shell.HookScript(shell.Zsh, "/usr/bin/ezp")
	if err != nil {
		t.Fatal(err)
	}
	// Same family as bash/posix: shared 1-based positional scanner.
	if !strings.Contains(zsh, `__ezp_scan`) {
		t.Fatalf("zsh hook should share posix scanner:\n%s", zsh)
	}
}

func TestFishAndPowerShellEmitShellFlag(t *testing.T) {
	fish, err := shell.HookScript(shell.Fish, "/usr/bin/ezp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fish, "--shell fish") {
		t.Fatalf("fish hook must pass --shell fish:\n%s", fish)
	}
	if !strings.Contains(fish, `__ezp_scan`) {
		t.Fatalf("fish hook must scan globals before on/off:\n%s", fish)
	}
	if !strings.Contains(fish, `__ezp_bool_false`) {
		t.Fatalf("fish hook must parse bool assignments:\n%s", fish)
	}
	ps, err := shell.HookScript(shell.PowerShell, `C:\ezp.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ps, "--shell") || !strings.Contains(ps, "powershell") {
		t.Fatalf("powershell hook must pass --shell powershell:\n%s", ps)
	}
	if !strings.Contains(ps, "Get-EzpExecForward") {
		t.Fatalf("powershell hook must place -- after exec flags:\n%s", ps)
	}
	if !strings.Contains(ps, "Get-EzpScan") {
		t.Fatalf("powershell hook must scan globals before on/off:\n%s", ps)
	}
	if !strings.Contains(ps, `--quiet=*`) || !strings.Contains(ps, `-q=*`) {
		t.Fatalf("powershell exec globals must accept --quiet=false:\n%s", ps)
	}
}

func TestNuHookUsesJSON(t *testing.T) {
	script, err := shell.HookScript(shell.Nu, "/usr/local/bin/ezp")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "let bin = `/usr/local/bin/ezp`") {
		t.Fatalf("nuQuote must not use backticks: %s", script)
	}
	if !strings.Contains(script, `let bin = "/usr/local/bin/ezp"`) {
		t.Fatalf("expected string literal bin: %s", script)
	}
	if !strings.Contains(script, "--json | complete") {
		t.Fatalf("nu hook should append --json last:\n%s", script)
	}
	if !strings.Contains(script, "entries_to_record") {
		t.Fatalf("nu hook must load env entries:\n%s", script)
	}
	if !strings.Contains(script, "prefer_lower") || !strings.Contains(script, "http_proxy") {
		t.Fatalf("nu hook must prefer lowercase proxy keys:\n%s", script)
	}
	if !strings.Contains(script, "def scan") {
		t.Fatalf("nu hook must scan globals before on/off:\n%s", script)
	}
}

func TestDetect(t *testing.T) {
	cases := map[string]shell.Kind{
		"bash": "bash", "zsh": "zsh", "sh": "sh", "posix": "sh",
		"fish": "fish", "pwsh": "powershell", "powershell": "powershell",
		"nu": "nu", "nushell": "nu", "cmd": "cmd",
	}
	for in, want := range cases {
		got, err := shell.Detect(in)
		if err != nil || got != want {
			t.Fatalf("%q => %q err=%v want %q", in, got, err, want)
		}
	}
	if _, err := shell.Detect("unknown-shell"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmitExportAndUnset(t *testing.T) {
	env := map[string]string{"http_proxy": "http://127.0.0.1:9", "no_proxy": "localhost"}
	sh := shell.EmitExport(shell.Posix, env)
	if !strings.Contains(sh, "export http_proxy=") || !strings.Contains(sh, "export no_proxy=") {
		t.Fatalf("posix export: %s", sh)
	}
	fish := shell.EmitExport(shell.Fish, env)
	if !strings.Contains(fish, "set -gx http_proxy") {
		t.Fatalf("fish: %s", fish)
	}
	ps := shell.EmitExport(shell.PowerShell, env)
	if !strings.Contains(ps, "$env:http_proxy") {
		t.Fatalf("ps: %s", ps)
	}
	cmd := shell.EmitExport(shell.Cmd, env)
	if !strings.Contains(cmd, `set "http_proxy=`) {
		t.Fatalf("cmd: %s", cmd)
	}
	amp := shell.EmitExport(shell.Cmd, map[string]string{"http_proxy": "http://x?a=1&b=2"})
	if !strings.Contains(amp, `set "http_proxy=http://x?a=1&b=2"`) {
		t.Fatalf("cmd amp: %s", amp)
	}

	keys := []string{"http_proxy", "HTTPS_PROXY"}
	if !strings.Contains(shell.EmitUnset(shell.Posix, keys), "unset http_proxy") {
		t.Fatal("posix unset")
	}
	if !strings.Contains(shell.EmitUnset(shell.Fish, keys), "set -e http_proxy; or true") {
		t.Fatal("fish unset")
	}
	if !strings.Contains(shell.EmitUnset(shell.Fish, keys), "true\n") {
		t.Fatal("fish unset must end with true")
	}
	if !strings.Contains(shell.EmitUnset(shell.PowerShell, keys), "Remove-Item Env:http_proxy") {
		t.Fatal("ps unset")
	}
}

func TestHookScriptAllShells(t *testing.T) {
	for _, kind := range []shell.Kind{shell.Bash, shell.Zsh, shell.Posix, shell.Fish, shell.PowerShell, shell.Nu} {
		script, err := shell.HookScript(kind, "/usr/local/bin/ezp")
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if !strings.Contains(script, "/usr/local/bin/ezp") {
			t.Fatalf("%s missing bin path: %s", kind, script)
		}
		if !strings.Contains(script, "on") || !strings.Contains(script, "off") {
			t.Fatalf("%s missing on/off: %s", kind, script)
		}
	}
	if _, err := shell.HookScript(shell.Cmd, "ezp"); err == nil {
		t.Fatal("cmd hook should fail")
	}
}

func TestQuotesHandleSpecialChars(t *testing.T) {
	env := map[string]string{"http_proxy": "http://u:p'ass@127.0.0.1:9"}
	out := shell.EmitExport(shell.Posix, env)
	if !strings.Contains(out, "export http_proxy=") {
		t.Fatalf("%s", out)
	}
}
