package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-switch-cli/internal/shell"
)

func TestUpsertAndStripBlock(t *testing.T) {
	block := MarkerBegin + "\nexport FOO=1\n" + MarkerEnd + "\n"
	next, changed, err := upsertBlock("existing\n", block)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(next, MarkerBegin) || !strings.Contains(next, "existing") {
		t.Fatalf("bad content: %q", next)
	}
	_, changed2, err := upsertBlock(next, block)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatal("idempotent write should not change")
	}
	stripped, ok, err := stripBlock(next)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || strings.Contains(stripped, MarkerBegin) {
		t.Fatalf("strip failed: %q", stripped)
	}
}

func TestKindFromSHELL(t *testing.T) {
	cases := map[string]shell.Kind{
		"/bin/zsh":                               shell.Zsh,
		"/usr/local/bin/bash":                    shell.Bash,
		`C:\Program Files\PowerShell\7\pwsh.exe`: shell.PowerShell,
	}
	for in, want := range cases {
		k, ok := kindFromSHELL(in)
		if !ok || k != want {
			t.Fatalf("%q => %q ok=%v want %q", in, k, ok, want)
		}
	}
}

func TestWriteSourceLineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte("# myrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	block, err := wrapSourceBlock(shell.Zsh)
	if err != nil {
		t.Fatal(err)
	}
	action, err := writeHook(path, block, false, true)
	if err != nil || action != "updated" {
		t.Fatalf("action=%s err=%v", action, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "eps Shell Integration") || strings.Contains(body, "eps()") {
		t.Fatalf("expected source line only: %q", body)
	}
	action, err = writeHook(path, block, false, true)
	if err != nil || action != "skipped" {
		t.Fatalf("second write action=%s err=%v", action, err)
	}
	action, err = removeHook(path, false)
	if err != nil || action != "removed" {
		t.Fatalf("remove action=%s err=%v", action, err)
	}
}

func TestWriteHookFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	path, err := hookFilePath(shell.Zsh)
	if err != nil {
		t.Fatal(err)
	}
	content := wrapHookFile("eps() { :; }\n")
	action, err := writeHookFile(path, content, false)
	if err != nil || action != "written" {
		t.Fatalf("action=%s err=%v", action, err)
	}
	action, err = writeHookFile(path, content, false)
	if err != nil || action != "skipped" {
		t.Fatalf("second write action=%s err=%v", action, err)
	}
	action, err = removeHookFile(path, false)
	if err != nil || action != "removed" {
		t.Fatalf("remove action=%s err=%v", action, err)
	}
}

func TestLegacyInlineBlockMigratesToSourceLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	zshrc := filepath.Join(dir, ".zshrc")
	legacy := "# keep\n\n" + MarkerBegin + "\n# easy-proxy-switch-cli shell hook\neps() { :; }\n" + MarkerEnd + "\n"
	if err := os.WriteFile(zshrc, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Run(Options{
		Shells:        []string{"zsh"},
		BinPath:       "/tmp/fake-eps",
		CreateMissing: true,
		InitConfig:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, "eps()") {
		t.Fatalf("legacy inline hook should be replaced: %q", body)
	}
	if !strings.Contains(body, "eps Shell Integration") {
		t.Fatalf("expected source line: %q", body)
	}
	if !strings.Contains(body, "# keep") {
		t.Fatalf("user content lost: %q", body)
	}
	hookPath, err := hookFilePath(shell.Zsh)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hookPath); err != nil {
		t.Fatalf("hook file missing: %v (targets=%+v)", err, res.Targets)
	}
}

func TestIncompleteBlockRefused(t *testing.T) {
	broken := MarkerBegin + "\neps() { :; }\n# user stuff\n"
	_, _, err := stripBlock(broken)
	if err == nil {
		t.Fatal("expected incomplete block error")
	}
	_, _, err = upsertBlock(broken, MarkerBegin+"\nok\n"+MarkerEnd+"\n")
	if err == nil {
		t.Fatal("upsert should refuse incomplete block")
	}
	onlyEnd := "keep\n" + MarkerEnd + "\n"
	_, _, err = upsertBlock(onlyEnd, MarkerBegin+"\nok\n"+MarkerEnd+"\n")
	if err == nil {
		t.Fatal("upsert should refuse end-only markers")
	}
	dup := MarkerBegin + "\na\n" + MarkerEnd + "\n" + MarkerBegin + "\nb\n" + MarkerEnd + "\n"
	_, _, err = upsertBlock(dup, MarkerBegin+"\nok\n"+MarkerEnd+"\n")
	if err == nil {
		t.Fatal("upsert should refuse duplicate blocks")
	}
}

func TestZDOTDIRAndXDGConfigPaths(t *testing.T) {
	home := t.TempDir()
	zdot := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ZDOTDIR", zdot)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	zshPaths := primaryPaths(shell.Zsh, home)
	if len(zshPaths) != 1 || zshPaths[0] != filepath.Join(zdot, ".zshrc") {
		t.Fatalf("zsh paths: %v", zshPaths)
	}
	fishPaths := primaryPaths(shell.Fish, home)
	wantFish := filepath.Join(xdg, "fish", "config.fish")
	if len(fishPaths) != 1 || fishPaths[0] != wantFish {
		t.Fatalf("fish paths: %v want %s", fishPaths, wantFish)
	}
	nuPaths := primaryPaths(shell.Nu, home)
	wantNu := filepath.Join(xdg, "nushell", "config.nu")
	if len(nuPaths) != 1 || nuPaths[0] != wantNu {
		t.Fatalf("nu paths: %v want %s", nuPaths, wantNu)
	}
}

func TestCreateMissingFalseDoesNotCreate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	missing := filepath.Join(home, ".zshrc")
	res, err := Run(Options{
		Shells:        []string{"zsh"},
		BinPath:       "/tmp/fake-eps",
		CreateMissing: false,
		InitConfig:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Fatalf(".zshrc should not be created: %v", statErr)
	}
	if len(res.Targets) == 0 {
		t.Fatal("expected a target")
	}
	for _, tr := range res.Targets {
		if tr.Action != "skipped" {
			t.Fatalf("action=%s want skipped (target=%+v)", tr.Action, tr)
		}
		if tr.Error != "" {
			t.Fatalf("soft skip should not error: %s", tr.Error)
		}
	}
}

func TestRunReturnsErrorOnTargetFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Make .zshrc a directory so writeHook fails.
	zshrc := filepath.Join(home, ".zshrc")
	if err := os.Mkdir(zshrc, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Run(Options{
		Shells:        []string{"zsh"},
		BinPath:       "/tmp/fake-eps",
		CreateMissing: true,
		InitConfig:    false,
	})
	if err == nil {
		t.Fatal("expected error when target path is a directory")
	}
}
