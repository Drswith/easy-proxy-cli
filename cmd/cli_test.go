package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/drswith/easy-proxy-cli/cmd"
	"github.com/drswith/easy-proxy-cli/internal/apperr"
	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/output"
)

func capture(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	ro, wo, _ := os.Pipe()
	re, we, _ := os.Pipe()
	os.Stdout, os.Stderr = wo, we
	defer func() {
		os.Stdout, os.Stderr = oldOut, oldErr
	}()

	// Read pipes concurrently to avoid deadlock when a child (ezp exec)
	// writes enough output to fill the OS pipe buffer.
	var bo, be bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&bo, ro) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&be, re) }()

	runErr := cmd.ExecuteArgs(args)
	_ = wo.Close()
	_ = we.Close()
	wg.Wait()
	return bo.String(), be.String(), runErr
}

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	// Windows UserHomeDir uses USERPROFILE, not HOME.
	t.Setenv("USERPROFILE", home)
}

func TestOffClearsUppercaseByDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	// Config with uppercase mirroring disabled should not affect off defaults.
	cfg := config.Default()
	cfg.Extras.MirrorUppercase = false
	cfg.Extras.NodeUseEnvProxy = false
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	out, _, err := capture(t, "off", "--emit", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NODE_USE_ENV_PROXY"} {
		if !strings.Contains(out, "unset "+k) {
			t.Fatalf("expected unset %s in:\n%s", k, out)
		}
	}
}

func TestMisconfigExitCode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	err := cmd.ExecuteArgs([]string{"on", "--mode", "invalid", "--quiet"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.IsMisconfig(err) {
		t.Fatalf("want misconfig, got %v", err)
	}
	if code := cmd.ExitCodeFor(err); code != output.ExitMisconfig {
		t.Fatalf("exit=%d", code)
	}
}

func TestCobraUsageIsMisconfig(t *testing.T) {
	err := cmd.ExecuteArgs([]string{"on", "--not-a-real-flag"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.IsMisconfig(err) {
		t.Fatalf("want misconfig, got %v", err)
	}
	if code := cmd.ExitCodeFor(err); code != output.ExitMisconfig {
		t.Fatalf("exit=%d", code)
	}
}

func TestSchemaJSON(t *testing.T) {
	out, _, err := capture(t, "schema")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "ezp" {
		t.Fatalf("%v", m)
	}
}

func TestOnEmitAndJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	out, _, err := capture(t, "on", "--emit", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "export http_proxy=") {
		t.Fatalf("stdout=%s", out)
	}
	out, _, err = capture(t, "on", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	env := m["env"].(map[string]any)
	if env["http_proxy"] == nil {
		t.Fatalf("%v", m)
	}
}

func TestOffEmit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	out, _, err := capture(t, "off", "--emit", "--quiet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unset http_proxy") {
		t.Fatalf("%s", out)
	}
}

func TestEnvFormats(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	out, _, err := capture(t, "env", "--format=dotenv")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http_proxy=") || strings.Contains(out, "export ") {
		t.Fatalf("%s", out)
	}
	out, _, err = capture(t, "env", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
}

func TestConfigLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	out, _, err := capture(t, "config", "init")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "config.toml") {
		t.Fatalf("%s", out)
	}
	_, _, err = capture(t, "config", "set", "default", "http", "http://127.0.0.1:9999")
	if err != nil {
		t.Fatal(err)
	}
	out, _, err = capture(t, "config", "show", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "9999") {
		t.Fatalf("%s", out)
	}
	out, _, err = capture(t, "profiles", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default") {
		t.Fatalf("%s", out)
	}
}

func TestHookAndSetupDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("SHELL", "/bin/zsh")

	out, _, err := capture(t, "hook", "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ezp()") {
		t.Fatalf("%s", out)
	}

	out, _, err = capture(t, "setup", "--dry-run", "--shell", "zsh", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	targets := m["targets"].([]any)
	if len(targets) == 0 {
		t.Fatalf("%v", m)
	}
}

func TestSetupWritesAndUninstalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("SHELL", "/bin/zsh")
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("# keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := capture(t, "setup", "--shell", "zsh", "--bin", "/tmp/fake-ezp")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "easy-proxy-cli") || !strings.Contains(body, "# keep") {
		t.Fatalf("%s", body)
	}

	_, _, err = capture(t, "setup", "--uninstall", "--shell", "zsh")
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "easy-proxy-cli") {
		t.Fatalf("hook not removed: %s", data)
	}
}

func TestStatusAndExec(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	t.Setenv("http_proxy", "http://example:1")
	out, _, err := capture(t, "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http_proxy") {
		t.Fatalf("%s", out)
	}

	// Prefer a small child so we never fill the stdout pipe (Windows CI env is huge).
	// exec injects the default profile (http://127.0.0.1:7897), not ambient env.
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"exec", "--", "cmd", "/c", "echo", "http_proxy=%http_proxy%"}
	} else {
		args = []string{"exec", "--", "sh", "-c", `printf '%s\n' "http_proxy=$http_proxy"`}
	}
	out, _, err = capture(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http_proxy=http://127.0.0.1:7897") {
		t.Fatalf("exec missing proxy: %s", out)
	}
}

func TestOnHostPortOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EASY_PROXY_HOME", dir)
	out, _, err := capture(t, "on", "--json", "--host", "10.0.0.2", "--port", "8888", "--no-node")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "10.0.0.2:8888") {
		t.Fatalf("%s", out)
	}
}
