package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/drswith/easy-proxy-cli/internal/apperr"
	"github.com/pelletier/go-toml/v2"
)

const (
	DirName       = ".easy-proxy"
	ConfigName    = "config.toml"
	ConfigVersion = 1
)

// Config is the on-disk ~/.easy-proxy/config.toml schema.
type Config struct {
	Version        int                `toml:"version" json:"version"`
	DefaultProfile string             `toml:"default_profile" json:"default_profile"`
	Profiles       map[string]Profile `toml:"profiles" json:"profiles"`
	Extras         Extras             `toml:"extras" json:"extras"`
}

// Profile describes one named proxy endpoint set.
type Profile struct {
	HTTP    string `toml:"http" json:"http"`
	HTTPS   string `toml:"https" json:"https"`
	Socks   string `toml:"socks" json:"socks"`
	NoProxy string `toml:"no_proxy" json:"no_proxy"`
}

// Extras toggles language/runtime sugar.
type Extras struct {
	MirrorUppercase  bool   `toml:"mirror_uppercase" json:"mirror_uppercase"`
	NodeUseEnvProxy  bool   `toml:"node_use_env_proxy" json:"node_use_env_proxy"`
	NodeExtraCACerts string `toml:"node_extra_ca_certs" json:"node_extra_ca_certs,omitempty"`
}

// raw* types distinguish omitted TOML fields from explicit zero values.
type rawConfig struct {
	Version        *int                  `toml:"version"`
	DefaultProfile *string               `toml:"default_profile"`
	Profiles       map[string]rawProfile `toml:"profiles"`
	Extras         *rawExtras            `toml:"extras"`
}

type rawProfile struct {
	HTTP    *string `toml:"http"`
	HTTPS   *string `toml:"https"`
	Socks   *string `toml:"socks"`
	NoProxy *string `toml:"no_proxy"`
}

type rawExtras struct {
	MirrorUppercase  *bool   `toml:"mirror_uppercase"`
	NodeUseEnvProxy  *bool   `toml:"node_use_env_proxy"`
	NodeExtraCACerts *string `toml:"node_extra_ca_certs"`
}

// Default returns a sensible Clash/V2Ray local mixed-port profile.
func Default() Config {
	return Config{
		Version:        ConfigVersion,
		DefaultProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				HTTP:    "http://127.0.0.1:7897",
				HTTPS:   "http://127.0.0.1:7897",
				Socks:   "socks5://127.0.0.1:7897",
				NoProxy: "localhost,127.0.0.1,::1,.local",
			},
		},
		Extras: Extras{
			MirrorUppercase: true,
			NodeUseEnvProxy: true,
		},
	}
}

func Dir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("EASY_PROXY_HOME")); v != "" {
		return expandHome(v), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigName), nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Load reads config from disk. Missing file returns Default without writing.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Config{}, apperr.Misconfig(fmt.Errorf("parse %s: %w", path, err))
	}
	return mergeRaw(raw), nil
}

func mergeRaw(raw rawConfig) Config {
	cfg := Default()
	if raw.Version != nil {
		cfg.Version = *raw.Version
	}
	if raw.DefaultProfile != nil && *raw.DefaultProfile != "" {
		cfg.DefaultProfile = *raw.DefaultProfile
	}
	if raw.Profiles != nil {
		cfg.Profiles = map[string]Profile{}
		for name, rp := range raw.Profiles {
			p := Profile{}
			if rp.HTTP != nil {
				p.HTTP = *rp.HTTP
			}
			if rp.HTTPS != nil {
				p.HTTPS = *rp.HTTPS
			} else {
				p.HTTPS = p.HTTP
			}
			if rp.Socks != nil {
				p.Socks = *rp.Socks
			}
			if rp.NoProxy != nil {
				p.NoProxy = *rp.NoProxy // may be ""
			} else {
				p.NoProxy = Default().Profiles["default"].NoProxy
			}
			cfg.Profiles[name] = p
		}
	}
	if raw.Extras != nil {
		if raw.Extras.MirrorUppercase != nil {
			cfg.Extras.MirrorUppercase = *raw.Extras.MirrorUppercase
		}
		if raw.Extras.NodeUseEnvProxy != nil {
			cfg.Extras.NodeUseEnvProxy = *raw.Extras.NodeUseEnvProxy
		}
		if raw.Extras.NodeExtraCACerts != nil {
			cfg.Extras.NodeExtraCACerts = *raw.Extras.NodeExtraCACerts
		}
	}
	if cfg.Version == 0 {
		cfg.Version = ConfigVersion
	}
	return cfg
}

// LoadOrCreate ensures the config directory and file exist.
func LoadOrCreate() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := Save(cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	return Load()
}

func Save(cfg Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfg.Version = ConfigVersion
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, ConfigName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	// Tighten perms even when overwriting an older 0644 file (may contain credentials).
	return os.Chmod(path, 0o600)
}

func (c Config) Profile(name string) (Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	p, ok := c.Profiles[name]
	if !ok {
		names := make([]string, 0, len(c.Profiles))
		for n := range c.Profiles {
			names = append(names, n)
		}
		return Profile{}, apperr.Misconfigf("profile %q not found (available: %s)", name, strings.Join(names, ", "))
	}
	if p.HTTPS == "" {
		p.HTTPS = p.HTTP
	}
	return p, nil
}

func (c Config) ListProfiles() []string {
	out := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
