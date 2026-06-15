package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBDsn          string
	ServerPort     string
	APIKey         string
	HTTPTimeoutSec int
}

func Load() (*Config, error) {
	// Load .env if present; ignore error if file doesn't exist
	_ = godotenv.Load()

	cfg := &Config{
		DBDsn:          os.Getenv("DB_DSN"),
		ServerPort:     getEnvOrDefault("SERVER_PORT", "8080"),
		APIKey:         os.Getenv("API_KEY"),
		HTTPTimeoutSec: getEnvIntOrDefault("HTTP_TIMEOUT_SEC", 30),
	}

	if cfg.DBDsn == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API_KEY is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
