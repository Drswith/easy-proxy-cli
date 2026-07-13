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
	if !strings.Contains(cmd, "set http_proxy=") {
		t.Fatalf("cmd: %s", cmd)
	}

	keys := []string{"http_proxy", "HTTPS_PROXY"}
	if !strings.Contains(shell.EmitUnset(shell.Posix, keys), "unset http_proxy") {
		t.Fatal("posix unset")
	}
	if !strings.Contains(shell.EmitUnset(shell.Fish, keys), "set -e http_proxy") {
		t.Fatal("fish unset")
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
