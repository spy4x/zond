// Package config loads Zond configuration from environment variables or a YAML file.
//
// Resolution order (highest priority first):
//  1. ZOND_TARGETS env var — comma-separated name=url,name=url
//  2. ZOND_CONFIG_PATH env var or ./zond.yml (falls back to ./zond.yaml)
//
// ZOND_PORT env var overrides the port declared in YAML.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"

	"github.com/spy4x/zond/internal/probe"
)

const (
	// DefaultPort is the HTTP listen port when ZOND_PORT is unset.
	DefaultPort = 8080

	// DefaultConfigFilename is the primary YAML config filename.
	DefaultConfigFilename = "zond.yml"
)

// yamlTimeoutUnit is the unit YAML uses for per-target timeouts.
// Going with milliseconds (int) to match common healthcheck schemas
// (Prometheus, Gatus, k8s probes, etc.) — string durations like "3s"
// would force consumers to learn a parser-specific format.
const yamlTimeoutUnit = time.Millisecond

// Config is the resolved Zond configuration.
type Config struct {
	Port    int
	Targets []probe.Target
}

// Load resolves configuration from env + filesystem.
func Load() (*Config, error) {
	cfg := &Config{Port: DefaultPort}

	if raw := os.Getenv("ZOND_TARGETS"); raw != "" {
		targets, err := parseTargetsEnv(raw)
		if err != nil {
			return nil, err
		}
		cfg.Targets = targets
		// ZOND_PORT may still override the default port even when targets come from env.
		if err := applyPort(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	path := resolveConfigPath()
	targets, port, err := loadYAML(path)
	if err != nil {
		return nil, err
	}
	cfg.Targets = targets
	if port > 0 {
		cfg.Port = port
	}
	// ZOND_PORT wins over YAML port for operational overrides.
	if err := applyPort(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyPort(cfg *Config) error {
	portStr := os.Getenv("ZOND_PORT")
	if portStr == "" {
		return nil
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("invalid ZOND_PORT=%q: must be 1-65535", portStr)
	}
	cfg.Port = p
	return nil
}

func resolveConfigPath() string {
	if p := os.Getenv("ZOND_CONFIG_PATH"); p != "" {
		return p
	}
	if _, err := os.Stat(DefaultConfigFilename); err == nil {
		return DefaultConfigFilename
	}
	return "zond.yaml"
}

func parseTargetsEnv(raw string) ([]probe.Target, error) {
	pairs := strings.Split(raw, ",")
	targets := make([]probe.Target, 0, len(pairs))
	seen := make(map[string]bool)
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			return nil, fmt.Errorf("invalid ZOND_TARGETS entry %q: expected name=url", pair)
		}
		name := strings.TrimSpace(pair[:eq])
		url := strings.TrimSpace(pair[eq+1:])
		if name == "" || url == "" {
			return nil, fmt.Errorf("invalid ZOND_TARGETS entry %q: empty name or url", pair)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate target name %q in ZOND_TARGETS", name)
		}
		seen[name] = true
		targets = append(targets, probe.Target{Name: name, URL: url, Timeout: probe.DefaultTimeout})
	}
	if len(targets) == 0 {
		return nil, errors.New("ZOND_TARGETS is empty")
	}
	return targets, nil
}

type yamlConfig struct {
	Port    int       `yaml:"port"`
	Targets []yamlTgt `yaml:"targets"`
}

type yamlTgt struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Timeout int    `yaml:"timeout"` // YAML unit: yamlTimeoutUnit (ms)
}

func loadYAML(path string) ([]probe.Target, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, errors.New("no config found: set ZOND_TARGETS env var or create a zond.yml file")
		}
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", path, err)
	}

	if len(yc.Targets) == 0 {
		return nil, 0, fmt.Errorf("no targets in %s: add a 'targets' array", path)
	}

	out := make([]probe.Target, 0, len(yc.Targets))
	seen := make(map[string]bool)
	for i, t := range yc.Targets {
		if t.Name == "" || t.URL == "" {
			return nil, 0, fmt.Errorf("target[%d] in %s: name and url are required", i, path)
		}
		if seen[t.Name] {
			return nil, 0, fmt.Errorf("duplicate target name %q in %s", t.Name, path)
		}
		seen[t.Name] = true
		timeout := probe.DefaultTimeout
		if t.Timeout > 0 {
			timeout = time.Duration(t.Timeout) * yamlTimeoutUnit
		}
		out = append(out, probe.Target{Name: t.Name, URL: t.URL, Timeout: timeout})
	}
	return out, yc.Port, nil
}
