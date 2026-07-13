package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-cli/internal/shell"
)

func TestUpsertAndStripBlock(t *testing.T) {
	block := MarkerBegin + "\nexport FOO=1\n" + MarkerEnd + "\n"
	next, changed := upsertBlock("existing\n", block)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(next, MarkerBegin) || !strings.Contains(next, "existing") {
		t.Fatalf("bad content: %q", next)
	}
	_, changed2 := upsertBlock(next, block)
	if changed2 {
		t.Fatal("idempotent write should not change")
	}
	stripped, ok := stripBlock(next)
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

func TestWriteHookRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte("# myrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	block := wrapBlock(shell.Zsh, "ezp() { :; }\n")
	action, err := writeHook(path, block, false, true)
	if err != nil || action != "updated" {
		t.Fatalf("action=%s err=%v", action, err)
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

func TestCreateMissingFalseDoesNotCreate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	missing := filepath.Join(home, ".zshrc")
	res, err := Run(Options{
		Shells:        []string{"zsh"},
		BinPath:       "/tmp/fake-ezp",
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
		BinPath:       "/tmp/fake-ezp",
		CreateMissing: true,
		InitConfig:    false,
	})
	if err == nil {
		t.Fatal("expected error when target path is a directory")
	}
}
