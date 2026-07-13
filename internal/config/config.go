package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg = cfg.withDefaults()
	return cfg, nil
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
	return os.WriteFile(path, data, 0o644)
}

func (c Config) withDefaults() Config {
	d := Default()
	if c.Version == 0 {
		c.Version = ConfigVersion
	}
	if c.DefaultProfile == "" {
		c.DefaultProfile = d.DefaultProfile
	}
	if c.Profiles == nil {
		c.Profiles = d.Profiles
	}
	for name, p := range c.Profiles {
		if p.HTTPS == "" {
			p.HTTPS = p.HTTP
		}
		if p.NoProxy == "" {
			p.NoProxy = d.Profiles["default"].NoProxy
		}
		c.Profiles[name] = p
	}
	return c
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
		return Profile{}, fmt.Errorf("profile %q not found (available: %s)", name, strings.Join(names, ", "))
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
