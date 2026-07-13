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

func TestE2ESetupHookBash(t *testing.T) {
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

	_, stderr, err := run(t, env, "setup", "--shell", "bash", "--bin", bin())
	if err != nil {
		t.Fatalf("setup: %v %s", err, stderr)
	}
	data, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "easy-proxy-cli") {
		t.Fatalf("bashrc not patched: %s", data)
	}

	// Simulate shell: source bashrc then call ezp on via the function.
	script := `
set -e
source "$HOME/.bashrc"
ezp on >/dev/null
test -n "$http_proxy"
ezp off >/dev/null
test -z "${http_proxy:-}"
`
	cmd := exec.Command("bash", "-lc", script)
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = home
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash hook: %v\n%s", err, out)
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
