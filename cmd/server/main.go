package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/config"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/router"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Connect(connectCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer pool.Close()

	cache := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer cache.Close()

	store, err := storage.NewMinioStore(storage.Config{
		Endpoint: cfg.StorageEndpoint, AccessKey: cfg.StorageAccessKey,
		SecretKey: cfg.StorageSecretKey, Bucket: cfg.StorageBucket, UseSSL: cfg.StorageUseSSL,
	})
	if err != nil {
		log.Fatalf("storage init error: %v", err)
	}

	oauthCtx := context.Background()
	googleVerifier, err := auth.NewGoogleVerifier(oauthCtx, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	if err != nil {
		log.Fatalf("oauth init error: %v", err)
	}
	userRepo := users.NewRepository(pool)
	cookieSecure := cfg.Env == "production"
	loginHandler := auth.NewLoginHandler(googleVerifier, cookieSecure)
	callbackHandler := auth.NewCallbackHandler(googleVerifier, userRepo, cfg.GoogleAllowedDomain, cfg.JWTSecret, cfg.FrontendURL, cookieSecure)

	handler := router.New(router.Deps{
		JWTSecret:    cfg.JWTSecret,
		Pool:         pool,
		Cache:        cache,
		Store:        store,
		AuthLogin:    loginHandler,
		AuthCallback: callbackHandler,
		FrontendURL:  cfg.FrontendURL,
	})

	log.Printf("iiitone-backend listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}
