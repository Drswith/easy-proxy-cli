package proxy_test

import (
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
