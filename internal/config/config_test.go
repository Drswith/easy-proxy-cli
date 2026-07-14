package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/drswith/easy-proxy-switch-cli/internal/config"
)

func TestSaveUsesPrivatePerms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	if err := config.Save(config.Default()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, config.ConfigName)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not honor Unix permission bits the same way.
	if runtime.GOOS == "windows" {
		return
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o want 0600", st.Mode().Perm())
	}
	// Overwrite a world-readable file and ensure chmod tightens it.
	if err := os.WriteFile(path, []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(config.Default()); err != nil {
		t.Fatal(err)
	}
	st, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("after overwrite perm=%o want 0600", st.Mode().Perm())
	}
}

func TestDefaultAndProfile(t *testing.T) {
	cfg := config.Default()
	if cfg.DefaultProfile != "default" {
		t.Fatalf("default profile: %q", cfg.DefaultProfile)
	}
	p, err := cfg.Profile("")
	if err != nil {
		t.Fatal(err)
	}
	if p.HTTP == "" || p.Socks == "" {
		t.Fatalf("empty urls: %+v", p)
	}
	if _, err := cfg.Profile("missing"); err == nil {
		t.Fatal("expected missing profile error")
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProfile != "default" {
		t.Fatalf("got %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, config.ConfigName)); !os.IsNotExist(err) {
		t.Fatal("Load should not create file")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)

	cfg := config.Default()
	cfg.DefaultProfile = "corp"
	cfg.Profiles["corp"] = config.Profile{
		HTTP:    "http://proxy.corp:8080",
		HTTPS:   "http://proxy.corp:8080",
		Socks:   "socks5://proxy.corp:1080",
		NoProxy: "intranet.corp",
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultProfile != "corp" {
		t.Fatalf("profile=%q", got.DefaultProfile)
	}
	p, err := got.Profile("corp")
	if err != nil {
		t.Fatal(err)
	}
	if p.HTTP != "http://proxy.corp:8080" {
		t.Fatalf("http=%q", p.HTTP)
	}
}

func TestLoadOrCreate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, config.ConfigName)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if cfg.Version != config.ConfigVersion {
		t.Fatalf("version=%d", cfg.Version)
	}
}

func TestExpandHomeAndDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EPS_HOME", dir)
	got, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("Dir=%q want %q", got, dir)
	}
}

func TestListProfilesSorted(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles["alpha"] = config.Profile{HTTP: "http://a"}
	cfg.Profiles["zeta"] = config.Profile{HTTP: "http://z"}
	names := cfg.ListProfiles()
	if len(names) < 3 {
		t.Fatalf("names=%v", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("not sorted: %v", names)
		}
	}
}

func TestHTTPSDefaultsToHTTP(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles["x"] = config.Profile{HTTP: "http://only-http:1"}
	p, err := cfg.Profile("x")
	if err != nil {
		t.Fatal(err)
	}
	if p.HTTPS != "http://only-http:1" {
		t.Fatalf("https=%q", p.HTTPS)
	}
}
