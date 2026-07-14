// Package config loads Zond configuration from environment variables or a YAML file.
//
// Resolution order (highest priority first):
//  1. ZOND_TARGETS env var — comma-separated name=url,name=url
//  2. ZOND_CONFIG_PATH env var or ./zond.yml (falls back to ./zond.yaml)
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

const (
	DefaultPort           = 8080
	DefaultProbeTimeout   = 5 * time.Second
	DefaultConfigFilename = "zond.yml"
)

// Target describes a single upstream to probe.
type Target struct {
	Name    string        `yaml:"name"`
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}

// Config is the resolved Zond configuration.
type Config struct {
	Port    int
	Targets []Target
}

// Load resolves configuration from env + filesystem.
// Returns an error if no targets are defined or input is malformed.
func Load() (*Config, error) {
	cfg := &Config{Port: DefaultPort}

	if portStr := os.Getenv("ZOND_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid ZOND_PORT=%q: must be 1-65535", portStr)
		}
		cfg.Port = p
	}

	if raw := os.Getenv("ZOND_TARGETS"); raw != "" {
		targets, err := parseTargetsEnv(raw)
		if err != nil {
			return nil, err
		}
		cfg.Targets = targets
		return cfg, nil
	}

	path := os.Getenv("ZOND_CONFIG_PATH")
	if path == "" {
		path = DefaultConfigFilename
		if _, err := os.Stat(DefaultConfigFilename); errors.Is(err, os.ErrNotExist) {
			path = "zond.yaml"
		}
	}

	targets, port, err := loadYAML(path)
	if err != nil {
		return nil, err
	}
	if port > 0 {
		cfg.Port = port
	}
	cfg.Targets = targets
	return cfg, nil
}

func parseTargetsEnv(raw string) ([]Target, error) {
	pairs := strings.Split(raw, ",")
	targets := make([]Target, 0, len(pairs))
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
		targets = append(targets, Target{Name: name, URL: url, Timeout: DefaultProbeTimeout})
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
	Timeout int    `yaml:"timeout"` // ms
}

func loadYAML(path string) ([]Target, int, error) {
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

	out := make([]Target, 0, len(yc.Targets))
	for i, t := range yc.Targets {
		if t.Name == "" || t.URL == "" {
			return nil, 0, fmt.Errorf("target[%d] in %s: name and url are required", i, path)
		}
		timeout := DefaultProbeTimeout
		if t.Timeout > 0 {
			timeout = time.Duration(t.Timeout) * time.Millisecond
		}
		out = append(out, Target{Name: t.Name, URL: t.URL, Timeout: timeout})
	}
	return out, yc.Port, nil
}
