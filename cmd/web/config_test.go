package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("LoadConfig with no args: %v", err)
	}
	def := DefaultConfig()
	if cfg.HTTP.Port != def.HTTP.Port {
		t.Errorf("HTTP.Port = %d, want %d", cfg.HTTP.Port, def.HTTP.Port)
	}
	if cfg.HTTP.TrustProxy != def.HTTP.TrustProxy {
		t.Errorf("HTTP.TrustProxy = %v, want %v", cfg.HTTP.TrustProxy, def.HTTP.TrustProxy)
	}
	if cfg.RateLimits.MsgRPS != def.RateLimits.MsgRPS {
		t.Errorf("RateLimits.MsgRPS = %v, want %v", cfg.RateLimits.MsgRPS, def.RateLimits.MsgRPS)
	}
	if cfg.RateLimits.IdleTTL != def.RateLimits.IdleTTL {
		t.Errorf("RateLimits.IdleTTL = %v, want %v", cfg.RateLimits.IdleTTL, def.RateLimits.IdleTTL)
	}
}

func TestJSONOverridesDefaults(t *testing.T) {
	path := writeConfigFile(t, `{"http":{"port":9000}}`)
	cfg, err := LoadConfig([]string{"--config", path})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTP.Port != 9000 {
		t.Errorf("HTTP.Port = %d, want 9000", cfg.HTTP.Port)
	}
	// Field not in JSON must keep its default.
	if cfg.RateLimits.MsgBurst != DefaultConfig().RateLimits.MsgBurst {
		t.Errorf("MsgBurst should keep default, got %d", cfg.RateLimits.MsgBurst)
	}
}

func TestEnvOverridesJSON(t *testing.T) {
	path := writeConfigFile(t, `{"http":{"port":9000}}`)
	t.Setenv("GOCHESS__HTTP__PORT", "9999")
	cfg, err := LoadConfig([]string{"--config", path})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTP.Port != 9999 {
		t.Errorf("HTTP.Port = %d, want 9999 (env should win over JSON)", cfg.HTTP.Port)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv("GOCHESS__HTTP__PORT", "9999")
	cfg, err := LoadConfig([]string{"--http.port", "7777"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTP.Port != 7777 {
		t.Errorf("HTTP.Port = %d, want 7777 (flag should win over env)", cfg.HTTP.Port)
	}
}

func TestLegacyFlagAlias(t *testing.T) {
	cfg, err := LoadConfig([]string{"--port", "7777"})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTP.Port != 7777 {
		t.Errorf("HTTP.Port = %d, want 7777 (legacy --port alias)", cfg.HTTP.Port)
	}
}

func TestLegacyDatabaseURL(t *testing.T) {
	const dsn = "postgres://gochess:gochess@localhost/gochess"
	t.Setenv("DATABASE_URL", dsn)
	// Ensure the canonical var is genuinely absent (not just empty) so the
	// env provider does not overwrite the bridge value with "".
	if err := os.Unsetenv("GOCHESS__DB__URL"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	t.Cleanup(func() { os.Unsetenv("GOCHESS__DB__URL") }) //nolint:errcheck

	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DB.URL != dsn {
		t.Errorf("DB.URL = %q, want %q (DATABASE_URL should be promoted)", cfg.DB.URL, dsn)
	}
}

func TestLegacyDatabaseURL_CanonicalWins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://old/db")
	t.Setenv("GOCHESS__DB__URL", "postgres://new/db")

	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.DB.URL != "postgres://new/db" {
		t.Errorf("DB.URL = %q, want postgres://new/db (GOCHESS__DB__URL should win)", cfg.DB.URL)
	}
}

func TestMissingConfigFileIsNotError(t *testing.T) {
	_, err := LoadConfig([]string{"--config", "/tmp/nonexistent-gochess-config-abc123.json"})
	if err != nil {
		t.Errorf("missing config file should not be an error, got: %v", err)
	}
}

func TestMalformedConfigFileIsError(t *testing.T) {
	path := writeConfigFile(t, `{broken json`)
	_, err := LoadConfig([]string{"--config", path})
	if err == nil {
		t.Error("malformed config file should return an error")
	}
}

func TestDurationParse(t *testing.T) {
	path := writeConfigFile(t, `{"rate_limits":{"idle_ttl":"30m"}}`)
	cfg, err := LoadConfig([]string{"--config", path})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.RateLimits.IdleTTL != 30*time.Minute {
		t.Errorf("IdleTTL = %v, want 30m", cfg.RateLimits.IdleTTL)
	}
}

// writeConfigFile writes content to a temp file and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}
