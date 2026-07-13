package proxy

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/apperr"
	"github.com/drswith/easy-proxy-cli/internal/config"
)

// EnvMap is an ordered-friendly map of proxy-related environment variables.
type EnvMap map[string]string

// ResolveOptions controls how a profile becomes env vars.
type ResolveOptions struct {
	Profile          string
	HTTP             string
	HTTPS            string
	Socks            string
	NoProxy          string
	NoProxySet       bool // true when CLI explicitly set --no-proxy (including empty)
	Host             string
	Port             int
	Mode             string // mixed | http | socks
	MirrorUppercase  *bool
	NodeUseEnvProxy  *bool
	NodeExtraCACerts string
	DisableNode      bool
	DisableUppercase bool
}

// Resolved is the fully resolved proxy environment to apply.
type Resolved struct {
	Profile string            `json:"profile"`
	Mode    string            `json:"mode"`
	Env     map[string]string `json:"env"`
	Extras  map[string]string `json:"extras,omitempty"`
}

// Resolve builds the env map from config + CLI overrides.
func Resolve(cfg config.Config, opt ResolveOptions) (Resolved, error) {
	p, err := cfg.Profile(opt.Profile)
	if err != nil {
		return Resolved{}, err
	}

	httpURL := firstNonEmpty(opt.HTTP, p.HTTP)
	httpsURL := firstNonEmpty(opt.HTTPS, p.HTTPS, httpURL)
	socksURL := firstNonEmpty(opt.Socks, p.Socks)
	var noProxy string
	if opt.NoProxySet {
		noProxy = opt.NoProxy
	} else {
		noProxy = p.NoProxy
	}

	if opt.Host != "" || opt.Port != 0 {
		host := firstNonEmpty(opt.Host, "127.0.0.1")
		port := opt.Port
		if port == 0 {
			port = 7897
		}
		if port < 1 || port > 65535 {
			return Resolved{}, apperr.Misconfigf("invalid port %d (want 1..65535)", port)
		}
		httpURL = fmt.Sprintf("http://%s", net.JoinHostPort(host, strconv.Itoa(port)))
		httpsURL = httpURL
		socksURL = fmt.Sprintf("socks5://%s", net.JoinHostPort(host, strconv.Itoa(port)))
	}

	mode := strings.ToLower(firstNonEmpty(opt.Mode, "mixed"))
	switch mode {
	case "mixed", "http", "socks":
	default:
		return Resolved{}, apperr.Misconfigf("invalid mode %q (want mixed|http|socks)", mode)
	}

	if err := validateURL(httpURL, "http"); err != nil && mode != "socks" {
		return Resolved{}, apperr.Misconfig(err)
	}
	if err := validateURL(httpsURL, "https"); err != nil && mode != "socks" {
		return Resolved{}, apperr.Misconfig(err)
	}
	if err := validateURL(socksURL, "socks"); err != nil && mode != "http" {
		return Resolved{}, apperr.Misconfig(err)
	}

	env := EnvMap{}
	switch mode {
	case "http":
		env["http_proxy"] = httpURL
		env["https_proxy"] = httpsURL
		env["all_proxy"] = httpURL
	case "socks":
		env["http_proxy"] = socksURL
		env["https_proxy"] = socksURL
		env["all_proxy"] = socksURL
	default: // mixed
		env["http_proxy"] = httpURL
		env["https_proxy"] = httpsURL
		env["all_proxy"] = socksURL
	}
	// Always emit no_proxy (may be empty) so explicit clears override parent env.
	env["no_proxy"] = noProxy

	mirror := cfg.Extras.MirrorUppercase
	if opt.MirrorUppercase != nil {
		mirror = *opt.MirrorUppercase
	}
	if opt.DisableUppercase {
		mirror = false
	}
	if mirror {
		for _, k := range []string{"http_proxy", "https_proxy", "all_proxy", "no_proxy"} {
			if v, ok := env[k]; ok {
				env[strings.ToUpper(k)] = v
			}
		}
	}

	extras := map[string]string{}
	node := cfg.Extras.NodeUseEnvProxy
	if opt.NodeUseEnvProxy != nil {
		node = *opt.NodeUseEnvProxy
	}
	if opt.DisableNode {
		node = false
	}
	if node {
		env["NODE_USE_ENV_PROXY"] = "1"
		extras["NODE_USE_ENV_PROXY"] = "1"
	}
	ca := firstNonEmpty(opt.NodeExtraCACerts, cfg.Extras.NodeExtraCACerts)
	if ca != "" {
		env["NODE_EXTRA_CA_CERTS"] = ca
		extras["NODE_EXTRA_CA_CERTS"] = ca
	}

	profileName := opt.Profile
	if profileName == "" {
		profileName = cfg.DefaultProfile
	}

	return Resolved{
		Profile: profileName,
		Mode:    mode,
		Env:     env,
		Extras:  extras,
	}, nil
}

// OffKeys returns names that ezp manages (for unset).
func OffKeys(mirrorUppercase, node bool) []string {
	keys := []string{
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
	}
	if mirrorUppercase {
		keys = append(keys,
			"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		)
	}
	if node {
		keys = append(keys, "NODE_USE_ENV_PROXY", "NODE_EXTRA_CA_CERTS")
	}
	return keys
}

// StaleManagedKeys returns managed keys absent from env that should be cleared
// before applying a new on payload (so prior on flags do not linger).
func StaleManagedKeys(env map[string]string) []string {
	var stale []string
	for _, k := range OffKeys(true, true) {
		if _, ok := env[k]; !ok {
			stale = append(stale, k)
		}
	}
	return stale
}

// IsActive reports whether a non-empty http/https/all proxy URL is set.
// Alone, no_proxy / Node extras do not count as an active proxy.
func IsActive(cur map[string]string) bool {
	for _, k := range []string{
		"http_proxy", "https_proxy", "all_proxy",
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
	} {
		if v, ok := cur[k]; ok && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// CurrentFromOS reads currently set proxy-related vars from the process env.
func CurrentFromOS() map[string]string {
	keys := []string{
		"http_proxy", "https_proxy", "all_proxy", "no_proxy",
		"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"NODE_USE_ENV_PROXY", "NODE_EXTRA_CA_CERTS",
	}
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			out[k] = v
		}
	}
	return out
}

// SortedKeys returns keys in stable order for shell emission.
func SortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// EnvEntry is one environment binding for JSON output.
// A list of entries stays valid under case-insensitive JSON parsers
// (e.g. PowerShell ConvertFrom-Json) while preserving every key.
type EnvEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EnvEntries returns env as a stable key/value list (sorted by key).
func EnvEntries(env map[string]string) []EnvEntry {
	keys := SortedKeys(env)
	out := make([]EnvEntry, 0, len(keys))
	for _, k := range keys {
		out = append(out, EnvEntry{Key: k, Value: env[k]})
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func validateURL(raw, kind string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%s proxy URL is empty", kind)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid %s proxy URL %q: %w", kind, raw, err)
	}
	if u.Scheme == "" || u.Host == "" || u.Hostname() == "" {
		return fmt.Errorf("invalid %s proxy URL %q: need scheme://host:port", kind, raw)
	}
	scheme := strings.ToLower(u.Scheme)
	switch kind {
	case "socks":
		switch scheme {
		case "socks", "socks5", "socks5h":
		default:
			return fmt.Errorf("invalid %s proxy URL %q: scheme must be socks5/socks5h", kind, raw)
		}
	default: // http / https proxy endpoints
		switch scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("invalid %s proxy URL %q: scheme must be http/https/socks5/socks5h", kind, raw)
		}
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid %s proxy URL %q: port must be 1..65535", kind, raw)
		}
	}
	return nil
}

// ApplyToEnviron returns a copy of environ with managed proxy keys stripped,
// then Resolved env merged in. Stripping first ensures --no-uppercase / --no-node
// do not leave stale HTTP_PROXY / NODE_* from the parent process.
func ApplyToEnviron(base []string, env map[string]string) []string {
	drop := map[string]struct{}{}
	for _, k := range OffKeys(true, true) {
		drop[k] = struct{}{}
	}
	out := make([]string, 0, len(base)+len(env))
	index := map[string]int{}
	for _, kv := range base {
		eq := strings.IndexByte(kv, '=')
		if eq > 0 {
			if _, skip := drop[kv[:eq]]; skip {
				continue
			}
			index[kv[:eq]] = len(out)
		}
		out = append(out, kv)
	}
	for _, k := range SortedKeys(env) {
		entry := k + "=" + env[k]
		if i, ok := index[k]; ok {
			out[i] = entry
		} else {
			out = append(out, entry)
			index[k] = len(out) - 1
		}
	}
	return out
}
