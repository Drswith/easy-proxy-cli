package proxy_test

import (
	"os"
	"strings"
	"testing"

	"github.com/drswith/easy-proxy-cli/internal/config"
	"github.com/drswith/easy-proxy-cli/internal/proxy"
)

func TestResolveDefaultMixed(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["http_proxy"] != "http://127.0.0.1:7897" {
		t.Fatalf("http_proxy=%q", r.Env["http_proxy"])
	}
	if r.Env["all_proxy"] != "socks5://127.0.0.1:7897" {
		t.Fatalf("all_proxy=%q", r.Env["all_proxy"])
	}
	if r.Env["HTTP_PROXY"] != r.Env["http_proxy"] {
		t.Fatal("expected uppercase mirror")
	}
	if r.Env["NODE_USE_ENV_PROXY"] != "1" {
		t.Fatal("expected NODE_USE_ENV_PROXY")
	}
}

func TestResolveHostPortOverride(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{Host: "10.0.0.1", Port: 8080, DisableNode: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["http_proxy"] != "http://10.0.0.1:8080" {
		t.Fatalf("got %q", r.Env["http_proxy"])
	}
	if _, ok := r.Env["NODE_USE_ENV_PROXY"]; ok {
		t.Fatal("node should be disabled")
	}
}

func TestResolveSocksMode(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{Mode: "socks", DisableUppercase: true, DisableNode: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["http_proxy"] != "socks5://127.0.0.1:7897" {
		t.Fatalf("got %q", r.Env["http_proxy"])
	}
	if _, ok := r.Env["HTTP_PROXY"]; ok {
		t.Fatal("uppercase should be off")
	}
}

func TestResolveInvalidModeAndURL(t *testing.T) {
	cfg := config.Default()
	if _, err := proxy.Resolve(cfg, proxy.ResolveOptions{Mode: "weird"}); err == nil {
		t.Fatal("expected mode error")
	}
	if _, err := proxy.Resolve(cfg, proxy.ResolveOptions{HTTP: "not-a-url", Mode: "http"}); err == nil {
		t.Fatal("expected url error")
	}
}

func TestResolveHTTPMode(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{Mode: "http", DisableNode: true, DisableUppercase: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["all_proxy"] != r.Env["http_proxy"] {
		t.Fatalf("all_proxy should mirror http in http mode: %+v", r.Env)
	}
}

func TestResolveCLIOverridesProfile(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{
		HTTP:             "http://override:1",
		HTTPS:            "http://override:1",
		Socks:            "socks5://override:2",
		NoProxy:          "only.local",
		NoProxySet:       true,
		DisableNode:      true,
		DisableUppercase: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["http_proxy"] != "http://override:1" || r.Env["all_proxy"] != "socks5://override:2" {
		t.Fatalf("%+v", r.Env)
	}
	if r.Env["no_proxy"] != "only.local" {
		t.Fatalf("no_proxy=%q", r.Env["no_proxy"])
	}
}

func TestOffKeys(t *testing.T) {
	keys := proxy.OffKeys(true, true)
	joined := strings.Join(keys, ",")
	for _, k := range []string{"http_proxy", "HTTP_PROXY", "NODE_USE_ENV_PROXY"} {
		if !strings.Contains(joined, k) {
			t.Fatalf("missing %s in %v", k, keys)
		}
	}
}

func TestApplyToEnvironAndCurrent(t *testing.T) {
	t.Setenv("http_proxy", "http://set:1")
	cur := proxy.CurrentFromOS()
	if cur["http_proxy"] != "http://set:1" {
		t.Fatalf("%+v", cur)
	}
	base := []string{"PATH=/bin", "http_proxy=old", "HTTP_PROXY=OLD", "NODE_USE_ENV_PROXY=1", "FOO=bar"}
	out := proxy.ApplyToEnviron(base, map[string]string{"http_proxy": "new"})
	found := map[string]string{}
	for _, kv := range out {
		parts := strings.SplitN(kv, "=", 2)
		found[parts[0]] = parts[1]
	}
	if found["http_proxy"] != "new" || found["FOO"] != "bar" || found["PATH"] != "/bin" {
		t.Fatalf("%+v", found)
	}
	if _, ok := found["HTTP_PROXY"]; ok {
		t.Fatalf("stale HTTP_PROXY should be stripped: %+v", found)
	}
	if _, ok := found["NODE_USE_ENV_PROXY"]; ok {
		t.Fatalf("stale NODE_USE_ENV_PROXY should be stripped: %+v", found)
	}
	_ = os.Unsetenv
}

func TestResolveEmptyNoProxyOverride(t *testing.T) {
	cfg := config.Default()
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{
		NoProxySet:       true,
		NoProxy:          "",
		DisableNode:      true,
		DisableUppercase: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := r.Env["no_proxy"]; !ok || v != "" {
		t.Fatalf("expected explicit empty no_proxy, got %q ok=%v", v, ok)
	}
}

func TestResolveRejectsBadScheme(t *testing.T) {
	cfg := config.Default()
	if _, err := proxy.Resolve(cfg, proxy.ResolveOptions{HTTP: "ftp://proxy:21", Mode: "http"}); err == nil {
		t.Fatal("expected scheme error")
	}
}

func TestSortedKeysStable(t *testing.T) {
	keys := proxy.SortedKeys(map[string]string{"b": "1", "a": "2"})
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("%v", keys)
	}
}

func TestNodeExtraCA(t *testing.T) {
	cfg := config.Default()
	cfg.Extras.NodeExtraCACerts = "/ca.pem"
	r, err := proxy.Resolve(cfg, proxy.ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Env["NODE_EXTRA_CA_CERTS"] != "/ca.pem" {
		t.Fatalf("%+v", r.Env)
	}
}
