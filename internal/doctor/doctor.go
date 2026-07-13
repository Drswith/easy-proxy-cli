package doctor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CheckResult is one connectivity probe.
type CheckResult struct {
	Name    string `json:"name"`
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Report aggregates doctor checks.
type Report struct {
	OK     bool          `json:"ok"`
	Checks []CheckResult `json:"checks"`
}

// Run probes TCP reachability of proxy URLs and optionally an HTTPS fetch via proxy.
func Run(httpProxy, socksProxy string, probeURL string, timeout time.Duration) Report {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	var checks []CheckResult
	if httpProxy != "" {
		checks = append(checks, tcpCheck("http_proxy", httpProxy, timeout))
	}
	if socksProxy != "" && socksProxy != httpProxy {
		checks = append(checks, tcpCheck("all_proxy", socksProxy, timeout))
	}
	if probeURL != "" && httpProxy != "" {
		checks = append(checks, httpCheck(httpProxy, probeURL, timeout))
	}
	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
			break
		}
	}
	return Report{OK: ok, Checks: checks}
}

func tcpCheck(name, raw string, timeout time.Duration) CheckResult {
	u, err := url.Parse(raw)
	if err != nil {
		return CheckResult{Name: name, Target: raw, OK: false, Error: err.Error()}
	}
	host, err := dialHost(u)
	if err != nil {
		return CheckResult{Name: name, Target: raw, OK: false, Error: err.Error()}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return CheckResult{Name: name, Target: host, OK: false, Error: err.Error()}
	}
	_ = conn.Close()
	return CheckResult{
		Name:    name,
		Target:  host,
		OK:      true,
		Latency: time.Since(start).Round(time.Millisecond).String(),
	}
}

func dialHost(u *url.URL) (string, error) {
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	if u.Port() != "" {
		return net.JoinHostPort(u.Hostname(), u.Port()), nil
	}
	port := ""
	switch strings.ToLower(u.Scheme) {
	case "http":
		port = "80"
	case "https":
		port = "443"
	case "socks", "socks5", "socks5h":
		port = "1080"
	default:
		return "", fmt.Errorf("missing port in address %s", u.Host)
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

func httpCheck(proxyURL, probeURL string, timeout time.Duration) CheckResult {
	pu, err := url.Parse(proxyURL)
	if err != nil {
		return CheckResult{Name: "https_via_proxy", Target: probeURL, OK: false, Error: err.Error()}
	}
	transport := &http.Transport{Proxy: http.ProxyURL(pu)}
	client := &http.Client{Transport: transport, Timeout: timeout}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, probeURL, nil)
	if err != nil {
		return CheckResult{Name: "https_via_proxy", Target: probeURL, OK: false, Error: err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		// Some sites reject HEAD; fall back to GET briefly.
		if strings.Contains(err.Error(), "HEAD") {
			return CheckResult{Name: "https_via_proxy", Target: probeURL, OK: false, Error: err.Error()}
		}
		return CheckResult{Name: "https_via_proxy", Target: probeURL, OK: false, Error: err.Error()}
	}
	defer resp.Body.Close()
	// 407 means the proxy rejected the request (auth required) — not healthy.
	ok := resp.StatusCode > 0 && resp.StatusCode < 500 && resp.StatusCode != http.StatusProxyAuthRequired
	msg := ""
	if !ok {
		msg = fmt.Sprintf("status %d", resp.StatusCode)
	}
	return CheckResult{
		Name:    "https_via_proxy",
		Target:  probeURL,
		OK:      ok,
		Latency: time.Since(start).Round(time.Millisecond).String(),
		Error:   msg,
	}
}
