package doctor_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drswith/easy-proxy-cli/internal/doctor"
)

func TestTCPCheckDefaultPort(t *testing.T) {
	// Assert default-port resolution only; do not require :80 to be closed
	// (GitHub Windows runners often have something listening on 80).
	report := doctor.Run("http://127.0.0.1", "", "", 200*time.Millisecond)
	if len(report.Checks) == 0 {
		t.Fatal("expected checks")
	}
	if report.Checks[0].Target != "127.0.0.1:80" {
		t.Fatalf("target=%q want 127.0.0.1:80", report.Checks[0].Target)
	}
}

func TestTCPCheckSuccessAndFail(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	ok := doctor.Run("http://"+addr, "", "", 500*time.Millisecond)
	if !ok.OK || len(ok.Checks) != 1 || !ok.Checks[0].OK {
		t.Fatalf("expected ok: %+v", ok)
	}

	fail := doctor.Run("http://127.0.0.1:1", "", "", 200*time.Millisecond)
	if fail.OK {
		t.Fatalf("expected fail: %+v", fail)
	}
}

func TestHTTPViaProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			w.WriteHeader(http.StatusOK)
			return
		}
		target := r.URL.String()
		if !r.URL.IsAbs() {
			target = "http://" + r.Host + r.URL.RequestURI()
		}
		req, err := http.NewRequest(r.Method, target, nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
	}))
	proxy.Listener = ln
	proxy.Start()
	defer proxy.Close()

	report := doctor.Run(proxy.URL, "", upstream.URL, time.Second)
	if !report.OK {
		t.Fatalf("report=%+v", report)
	}
}

func TestHTTPViaProxyRejects407(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	proxy.Listener = ln
	proxy.Start()
	defer proxy.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	report := doctor.Run(proxy.URL, "", upstream.URL, time.Second)
	if report.OK {
		t.Fatalf("407 should not be healthy: %+v", report)
	}
	found := false
	for _, c := range report.Checks {
		if c.Name == "https_via_proxy" && !c.OK && strings.Contains(c.Error, "407") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected https_via_proxy 407 failure: %+v", report.Checks)
	}
}
