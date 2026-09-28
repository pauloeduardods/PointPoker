package config

import (
	"slices"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"API_PORT", "CORS_ORIGINS", "DB_HOST"} {
		t.Setenv(k, "")
	}
	cfg := Load()
	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want 8080", cfg.ServerPort)
	}
	want := []string{"http://localhost:5173", "http://localhost:3000"}
	if !slices.Equal(cfg.CORSOrigins, want) {
		t.Errorf("CORSOrigins = %v, want %v", cfg.CORSOrigins, want)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("API_PORT", "9999")
	t.Setenv("CORS_ORIGINS", " https://a.example , ,https://b.example ")
	cfg := Load()
	if cfg.ServerPort != "9999" {
		t.Errorf("ServerPort = %q", cfg.ServerPort)
	}
	if want := []string{"https://a.example", "https://b.example"}; !slices.Equal(cfg.CORSOrigins, want) {
		t.Errorf("CORSOrigins = %v, want %v", cfg.CORSOrigins, want)
	}
}
