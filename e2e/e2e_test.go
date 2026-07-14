package e2e_test

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Build binary once into temp-ish path under repo bin for e2e.
	root := findRepoRoot()
	binDir := filepath.Join(root, "bin")
	_ = os.MkdirAll(binDir, 0o755)
	bin := filepath.Join(binDir, "ezp-e2e")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/ezp")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	os.Setenv("EZP_E2E_BIN", bin)
	os.Exit(m.Run())
}

func findRepoRoot() string {
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return wd
}

func bin() string {
	return os.Getenv("EZP_E2E_BIN")
}

func run(t *testing.T, env []string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(bin(), args...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestE2ECoreFlow(t *testing.T) {
	home := t.TempDir()
	cfg := t.TempDir()
	env := []string{
		"HOME=" + home,
		"EASY_PROXY_HOME=" + cfg,
		"SHELL=/bin/bash",
	}

	out, _, err := run(t, env, "config", "init", "--json")
	if err != nil {
		t.Fatalf("init: %v %s", err, out)
	}

	out, _, err = run(t, env, "on", "--emit", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "export http_proxy=") {
		t.Fatalf("on emit: %s", out)
	}

	out, _, err = run(t, env, "env", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatal(err)
	}
	foundNode := false
	for _, e := range entries {
		if e.Key == "NODE_USE_ENV_PROXY" && e.Value == "1" {
			foundNode = true
			break
		}
	}
	if !foundNode {
		t.Fatalf("%v", entries)
	}

	out, _, err = run(t, env, "exec", "--", "printenv", "http_proxy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http://127.0.0.1:7897") {
		t.Fatalf("exec: %s", out)
	}

	out, _, err = run(t, env, "schema")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "ezp"`) && !strings.Contains(out, `"name":"ezp"`) {
		t.Fatalf("schema: %s", out)
	}
}

func TestE2ESetupHookShells(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix shell hook e2e")
	}

	type shellCase struct {
		name   string
		shell  string // ezp setup --shell
		interp string // interpreter to run
		rcRel  string
		source string // how to load the rc in the interpreter
		skipIf string // optional binary that must exist
	}
	cases := []shellCase{
		{name: "bash", shell: "bash", interp: "bash", rcRel: ".bashrc", source: `source "$HOME/.bashrc"`},
		{name: "dash", shell: "sh", interp: "dash", rcRel: ".profile", source: `. "$HOME/.profile"`, skipIf: "dash"},
		{name: "zsh", shell: "zsh", interp: "zsh", rcRel: ".zshrc", source: `source "$HOME/.zshrc"`, skipIf: "zsh"},
		{name: "fish", shell: "fish", interp: "fish", rcRel: ".config/fish/config.fish", source: `source "$HOME/.config/fish/config.fish"`, skipIf: "fish"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipIf != "" {
				if _, err := exec.LookPath(tc.skipIf); err != nil {
					t.Skipf("%s not installed", tc.skipIf)
				}
			}
			home := t.TempDir()
			cfg := t.TempDir()
			rc := filepath.Join(home, tc.rcRel)
			if err := os.MkdirAll(filepath.Dir(rc), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(rc, []byte("# base\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			env := []string{
				"HOME=" + home,
				"EASY_PROXY_HOME=" + cfg,
				"SHELL=" + tc.interp,
			}
			_, stderr, err := run(t, env, "setup", "--shell", tc.shell, "--bin", bin())
			if err != nil {
				t.Fatalf("setup: %v %s", err, stderr)
			}
			data, err := os.ReadFile(rc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "easy-proxy-cli") {
				t.Fatalf("rc not patched: %s", data)
			}

			var script string
			switch tc.name {
			case "fish":
				script = tc.source + `
ezp on >/dev/null; or exit $status
test -n "$http_proxy"; or exit 1
ezp -q on >/dev/null; or exit $status
test -n "$http_proxy"; or exit 1
ezp off >/dev/null; or exit $status
not set -q http_proxy; or exit 1
ezp on --json >/dev/null; or exit $status
`
			case "zsh":
				script = `
set -e
` + tc.source + `
ezp on >/dev/null
test -n "$http_proxy"
ezp -q on >/dev/null
test -n "$http_proxy"
ezp on -qj=false >/dev/null
test -n "$http_proxy"
ezp off >/dev/null
test -z "${http_proxy:-}"
out="$(ezp on --json)"
print -r -- "$out" | grep -q '"key"'
`
			default: // bash, dash
				script = `
set -e
` + tc.source + `
ezp on >/dev/null
test -n "$http_proxy"
ezp -q on >/dev/null
test -n "$http_proxy"
ezp on -qj=false >/dev/null
test -n "$http_proxy"
ezp off >/dev/null
test -z "${http_proxy:-}"
out="$(ezp on --json)"
printf '%s\n' "$out" | grep -q '"key"'
`
			}
			cmd := exec.Command(tc.interp, "-c", script)
			if tc.name == "zsh" {
				cmd = exec.Command(tc.interp, "-fc", script)
			}
			cmd.Env = append(os.Environ(), env...)
			cmd.Dir = home
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s hook: %v\n%s", tc.name, err, out)
			}
		})
	}
}

func TestE2EDoctorAgainstLocalListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	home := t.TempDir()
	cfg := t.TempDir()
	env := []string{"HOME=" + home, "EASY_PROXY_HOME=" + cfg}

	_, _, err = run(t, env, "config", "init")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = run(t, env, "config", "set", "default", "http", "http://"+addr)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = run(t, env, "config", "set", "default", "socks", "socks5://"+addr)
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := run(t, env, "doctor", "--json", "--url", "-")
	if err != nil {
		t.Fatalf("doctor: %v out=%s", err, out)
	}
	var report struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("json=%s err=%v", out, err)
	}
	if !report.OK {
		t.Fatalf("expected ok: %+v", report)
	}
	found := false
	for _, c := range report.Checks {
		if c.Name == "http_proxy" && c.OK {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected http_proxy ok: %+v", report)
	}
}

func TestE2ESetupHookPowerShell(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if runtime.GOOS == "windows" {
			pwsh, err = exec.LookPath("powershell")
		}
		if err != nil {
			t.Skip("pwsh/powershell not installed")
		}
	}
	home := t.TempDir()
	cfg := t.TempDir()
	env := []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"EASY_PROXY_HOME=" + cfg,
	}
	hookOut, _, err := run(t, env, "hook", "powershell")
	if err != nil {
		t.Fatal(err)
	}
	hookFile := filepath.Join(home, "ezp-hook.ps1")
	if err := os.WriteFile(hookFile, []byte(hookOut), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
$ErrorActionPreference = 'Stop'
. '` + hookFile + `'
ezp on
if (-not $env:http_proxy) { throw 'http_proxy missing after on' }
ezp -q on
if (-not $env:http_proxy) { throw 'http_proxy missing after -q on' }
ezp off
if ($env:http_proxy) { throw 'http_proxy still set after off' }
$json = ezp on --json | Out-String
if ($json -notmatch '"key"') { throw 'json missing key' }
`
	cmd := exec.Command(pwsh, "-NoProfile", "-Command", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell hook: %v\n%s", err, out)
	}
}

func TestE2ESetupHookNu(t *testing.T) {
	if _, err := exec.LookPath("nu"); err != nil {
		t.Skip("nu not installed")
	}
	home := t.TempDir()
	cfg := t.TempDir()
	env := []string{
		"HOME=" + home,
		"EASY_PROXY_HOME=" + cfg,
		// Inherited uppercase aliases must not survive ezp on (Nu prefers lowercase).
		"HTTP_PROXY=http://stale.example:9",
		"HTTPS_PROXY=http://stale.example:9",
		"ALL_PROXY=socks5://stale.example:9",
	}
	hookOut, _, err := run(t, env, "hook", "nu")
	if err != nil {
		t.Fatal(err)
	}
	hookFile := filepath.Join(home, "ezp-hook.nu")
	if err := os.WriteFile(hookFile, []byte(hookOut), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
source '` + hookFile + `'
ezp on
if ($env.http_proxy? | default "") == "" { error make {msg: "http_proxy missing after on"} }
if ($env.http_proxy | str contains "stale.example") { error make {msg: "stale lowercase proxy survived"} }
# Child processes must not see the inherited uppercase stale values.
let child = (^env | complete)
if ($child.stdout | str contains "stale.example") { error make {msg: $"stale uppercase survived in child env: ($child.stdout)"} }
ezp -q on
if ($env.http_proxy? | default "") == "" { error make {msg: "http_proxy missing after -q on"} }
ezp off
if ($env.http_proxy? | default "") != "" { error make {msg: "http_proxy still set after off"} }
`
	cmd := exec.Command("nu", "--no-config-file", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nu hook: %v\n%s", err, out)
	}
}

func TestE2EBashHookPropagatesFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash hook e2e is unix")
	}
	home := t.TempDir()
	cfg := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := []string{
		"HOME=" + home,
		"EASY_PROXY_HOME=" + cfg,
		"SHELL=/bin/bash",
	}
	_, _, err := run(t, env, "setup", "--shell", "bash", "--bin", bin())
	if err != nil {
		t.Fatal(err)
	}
	script := `
set -e
source "$HOME/.bashrc"
set +e
ezp on --profile does-not-exist >/dev/null 2>&1
status=$?
test "$status" -ne 0
`
	cmd := exec.Command("bash", "-lc", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook should propagate failure: %v\n%s", err, out)
	}
}

func TestE2EInstallScriptSyntax(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash -n")
	}
	root := findRepoRoot()
	cmd := exec.Command("bash", "-n", filepath.Join(root, "scripts/install.sh"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
}
