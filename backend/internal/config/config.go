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

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	BaseURL string // публичный URL сервера (для file ссылок в prod)

	StorageMode     string // "local" | "s3"
	StorageLocalDir string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Region         string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3UsePathStyle   bool

	WorkerCount int

	KieAPIKey  string // API ключ kie.ai для изображений и музыки (пусто = mock)
	FFmpegPath string // путь к ffmpeg для предобработки mashup-аудио
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:             getEnv("APP_ENV", "development"),
		AppPort:            getEnv("APP_PORT", "8080"),
		BaseURL:            getEnv("BASE_URL", ""),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:  getEnv("GOOGLE_REDIRECT_URI", ""),
		StorageMode:        getEnv("STORAGE_MODE", "local"),
		StorageLocalDir:    getEnv("STORAGE_LOCAL_DIR", "./data/uploads"),
		S3Endpoint:         getEnvAny([]string{"S3_ENDPOINT"}, ""),
		S3PublicEndpoint:   getEnvAny([]string{"S3_PUBLIC_ENDPOINT"}, ""),
		S3Region:           getEnvAny([]string{"S3_REGION"}, "us-east-1"),
		S3AccessKey:        getEnvAny([]string{"S3_ACCESS_KEY", "R2_ACCESS_KEY"}, ""),
		S3SecretKey:        getEnvAny([]string{"S3_SECRET_KEY", "R2_SECRET_KEY"}, ""),
		S3Bucket:           getEnvAny([]string{"S3_BUCKET", "R2_BUCKET"}, "fungreet"),
		S3UsePathStyle:     getEnvBool("S3_USE_PATH_STYLE", false),
		WorkerCount:        getEnvInt("WORKER_COUNT", 2),
		KieAPIKey:          getEnv("KIE_API_KEY", ""),
		FFmpegPath:         getEnv("FFMPEG_PATH", "ffmpeg"),
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

func getEnvAny(keys []string, fallback string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
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

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
