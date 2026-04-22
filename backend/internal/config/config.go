package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv  string
	AppPort string

	DatabaseURL string
	RedisURL    string

	JWTSecret string

	StorageMode     string // "local" | "r2"
	StorageLocalDir string

	R2AccountID  string
	R2AccessKey  string
	R2SecretKey  string
	R2Bucket     string

	WorkerCount int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		AppPort:         getEnv("APP_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		StorageMode:     getEnv("STORAGE_MODE", "local"),
		StorageLocalDir: getEnv("STORAGE_LOCAL_DIR", "./data/uploads"),
		R2AccountID:     getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKey:     getEnv("R2_ACCESS_KEY", ""),
		R2SecretKey:     getEnv("R2_SECRET_KEY", ""),
		R2Bucket:        getEnv("R2_BUCKET", "fungreet"),
		WorkerCount:     getEnvInt("WORKER_COUNT", 2),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func (c *Config) IsDev() bool {
	return c.AppEnv == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
