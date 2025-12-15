package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env      string
	LogLevel string

	DatabaseURL string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	PlaidClientID string
	PlaidSecret   string
	PlaidEnv      string

	WorkerConcurrency int
	LockTTL           time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:      getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DatabaseURL: getEnv("DATABASE_URL", ""),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		PlaidClientID: getEnv("PLAID_CLIENT_ID", ""),
		PlaidSecret:   getEnv("PLAID_SECRET", ""),
		PlaidEnv:      getEnv("PLAID_ENV", "sandbox"),

		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 5),
		LockTTL:           getEnvDuration("LOCK_TTL", 2*time.Minute),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.PlaidClientID == "" {
		return fmt.Errorf("PLAID_CLIENT_ID is required")
	}
	if c.PlaidSecret == "" {
		return fmt.Errorf("PLAID_SECRET is required")
	}

	validPlaidEnvs := map[string]bool{
		"sandbox":     true,
		"development": true,
		"production":  true,
	}
	if !validPlaidEnvs[c.PlaidEnv] {
		return fmt.Errorf("invalid PLAID_ENV: %s (must be sandbox, development, or production)", c.PlaidEnv)
	}

	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be at least 1")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	valStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		panic(fmt.Sprintf("env var %s must be an integer, got: %s", key, valStr))
	}
	return val
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	valStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		panic(fmt.Sprintf("env var %s must be a valid duration (e.g. 10s, 2m), got: %s", key, valStr))
	}
	return val
}
