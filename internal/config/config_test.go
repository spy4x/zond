package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromEnvTargets(t *testing.T) {
	t.Setenv("ZOND_TARGETS", "a=http://a:80/,b=http://b:8080/")
	t.Setenv("ZOND_PORT", "9090")
	t.Setenv("ZOND_CONFIG_PATH", "") // ignored — env targets win

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Port)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Targets))
	}
	if cfg.Targets[0].Name != "a" || cfg.Targets[0].URL != "http://a:80/" {
		t.Errorf("targets[0] = %+v", cfg.Targets[0])
	}
	if cfg.Targets[0].Timeout != DefaultProbeTimeout {
		t.Errorf("timeout = %v, want %v", cfg.Targets[0].Timeout, DefaultProbeTimeout)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zond.yml")
	body := `port: 7070
targets:
  - name: metube
    url: http://metube:8081/
    timeout: 3000
  - name: ollama
    url: http://ollama:11434/api/tags
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ZOND_TARGETS", "")
	t.Setenv("ZOND_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 7070 {
		t.Errorf("port = %d, want 7070", cfg.Port)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets = %d, want 2", len(cfg.Targets))
	}
	if cfg.Targets[0].Name != "metube" {
		t.Errorf("targets[0].Name = %q, want metube", cfg.Targets[0].Name)
	}
	if cfg.Targets[0].Timeout != 3*time.Second {
		t.Errorf("targets[0].Timeout = %v, want 3s", cfg.Targets[0].Timeout)
	}
	if cfg.Targets[1].Timeout != DefaultProbeTimeout {
		t.Errorf("targets[1].Timeout = %v, want default", cfg.Targets[1].Timeout)
	}
}

func TestLoadNoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZOND_TARGETS", "")
	t.Setenv("ZOND_CONFIG_PATH", "")
	t.Chdir(dir)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when no config present")
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("ZOND_TARGETS", "a=http://a")
	t.Setenv("ZOND_PORT", "not-a-number")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error on invalid port")
	}
}

func TestLoadInvalidTargetsEntry(t *testing.T) {
	t.Setenv("ZOND_TARGETS", "badentry,no-equals-here")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error on malformed entry")
	}
}
