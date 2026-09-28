// Package config loads application settings from environment variables.
package config

import (
	"os"
	"strings"
)

// Config holds all configuration for the application.
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	ServerPort string
	// CORSOrigins lists the origins allowed to call the API cross-origin.
	// A single "*" allows any origin.
	CORSOrigins []string
}

// Load reads configuration from environment variables with sensible defaults.
//
// GIN_MODE is honoured by gin itself and therefore not part of Config.
func Load() *Config {
	return &Config{
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "pointpoker"),
		DBPassword:  getEnv("DB_PASSWORD", "pointpoker"),
		DBName:      getEnv("DB_NAME", "pointpoker"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		ServerPort:  getEnv("API_PORT", "8080"),
		CORSOrigins: splitList(getEnv("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000")),
	}
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitList splits a comma-separated list, trimming blanks and dropping empty items.
func splitList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
