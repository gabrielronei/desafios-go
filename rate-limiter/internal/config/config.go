package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// TokenConfig holds rate limit configuration for a specific token.
type TokenConfig struct {
	MaxRequests   int
	BlockDuration time.Duration
}

// Config holds all application configuration.
type Config struct {
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	IPMaxRequests      int
	IPBlockDuration    time.Duration
	TokenMaxRequests   int
	TokenBlockDuration time.Duration
	TokenConfigs       map[string]TokenConfig
}

// Load reads configuration from environment variables.
// It also tries to load app.env (and the legacy .env) from the working directory;
// any missing file is silently ignored.
func Load() (*Config, error) {
	_ = godotenv.Load("app.env")
	_ = godotenv.Overload() // .env overrides app.env when present

	cfg := &Config{
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvInt("REDIS_DB", 0),
		IPMaxRequests:      getEnvInt("RATE_LIMIT_IP_MAX_REQUESTS", 10),
		IPBlockDuration:    getEnvDuration("RATE_LIMIT_IP_BLOCK_DURATION_SECONDS", 300),
		TokenMaxRequests:   getEnvInt("RATE_LIMIT_TOKEN_MAX_REQUESTS", 100),
		TokenBlockDuration: getEnvDuration("RATE_LIMIT_TOKEN_BLOCK_DURATION_SECONDS", 300),
		TokenConfigs:       make(map[string]TokenConfig),
	}

	if err := parseTokenConfigs(cfg); err != nil {
		return nil, fmt.Errorf("parsing token configs: %w", err)
	}

	return cfg, nil
}

// parseTokenConfigs parses TOKEN_RATE_LIMITS env var.
// Format: "token1:maxRequests:blockSeconds,token2:maxRequests:blockSeconds"
// Example: "abc123:100:300,xyz789:50:60"
func parseTokenConfigs(cfg *Config) error {
	raw := getEnv("TOKEN_RATE_LIMITS", "")
	if raw == "" {
		return nil
	}

	entries := strings.Split(raw, ",")
	for _, entry := range entries {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) != 3 {
			return fmt.Errorf("invalid TOKEN_RATE_LIMITS entry %q: expected format token:maxRequests:blockSeconds", entry)
		}

		token := strings.TrimSpace(parts[0])
		maxReq, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("invalid max_requests in TOKEN_RATE_LIMITS for token %q: %w", token, err)
		}

		blockSecs, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			return fmt.Errorf("invalid block_seconds in TOKEN_RATE_LIMITS for token %q: %w", token, err)
		}

		cfg.TokenConfigs[token] = TokenConfig{
			MaxRequests:   maxReq,
			BlockDuration: time.Duration(blockSecs) * time.Second,
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallbackSeconds int) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(fallbackSeconds) * time.Second
}
