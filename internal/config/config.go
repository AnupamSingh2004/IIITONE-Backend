package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                string
	Env                 string
	DatabaseURL         string
	RedisAddr           string
	StorageEndpoint     string
	StorageAccessKey    string
	StorageSecretKey    string
	StorageBucket       string
	StorageUseSSL       bool
	GoogleClientID      string
	GoogleClientSecret  string
	GoogleAllowedDomain string
	GoogleRedirectURL   string
	JWTSecret           string
	FrontendURL         string
}

func Load() (Config, error) {
	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 getEnv("ENV", "development"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RedisAddr:           os.Getenv("REDIS_ADDR"),
		StorageEndpoint:     os.Getenv("STORAGE_ENDPOINT"),
		StorageAccessKey:    os.Getenv("STORAGE_ACCESS_KEY"),
		StorageSecretKey:    os.Getenv("STORAGE_SECRET_KEY"),
		StorageBucket:       os.Getenv("STORAGE_BUCKET"),
		StorageUseSSL:       os.Getenv("STORAGE_USE_SSL") == "true",
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleAllowedDomain: os.Getenv("GOOGLE_ALLOWED_DOMAIN"),
		GoogleRedirectURL:   os.Getenv("GOOGLE_REDIRECT_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		FrontendURL:         os.Getenv("FRONTEND_URL"),
	}

	required := map[string]string{
		"DATABASE_URL":          cfg.DatabaseURL,
		"REDIS_ADDR":            cfg.RedisAddr,
		"STORAGE_ENDPOINT":      cfg.StorageEndpoint,
		"GOOGLE_ALLOWED_DOMAIN": cfg.GoogleAllowedDomain,
		"JWT_SECRET":            cfg.JWTSecret,
	}
	for name, val := range required {
		if val == "" {
			return Config{}, fmt.Errorf("missing required env var: %s", name)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
