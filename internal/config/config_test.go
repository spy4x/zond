package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spy4x/zond/internal/probe"
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
	if cfg.Targets[0].Timeout != probe.DefaultTimeout {
		t.Errorf("timeout = %v, want %v", cfg.Targets[0].Timeout, probe.DefaultTimeout)
	}
	if cfg.Targets[0].Name != "a" {
		t.Errorf("cfg.Targets[0] type mismatch: %T", cfg.Targets[0])
	}
}

func TestLoadPortEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zond.yml")
	if err := os.WriteFile(path, []byte("port: 7070\ntargets:\n  - name: x\n    url: http://x\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ZOND_TARGETS", "")
	t.Setenv("ZOND_PORT", "9999")
	t.Setenv("ZOND_CONFIG_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("port = %d, want 9999 (ZOND_PORT wins over YAML)", cfg.Port)
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
	if cfg.Targets[1].Timeout != probe.DefaultTimeout {
		t.Errorf("targets[1].Timeout = %v, want default", cfg.Targets[1].Timeout)
	}
}

func TestLoadRejectsDuplicateNamesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "zond.yml")
	body := `targets:
  - name: dup
    url: http://a
  - name: dup
    url: http://b
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ZOND_TARGETS", "")
	t.Setenv("ZOND_CONFIG_PATH", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected duplicate-name error")
	}
}

func TestLoadRejectsDuplicateNamesEnv(t *testing.T) {
	t.Setenv("ZOND_TARGETS", "x=http://a/,x=http://b/")
	_, err := Load()
	if err == nil {
		t.Fatal("expected duplicate-name error")
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
