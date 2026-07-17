package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_AllVarsPresent(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "ak")
	t.Setenv("STORAGE_SECRET_KEY", "sk")
	t.Setenv("STORAGE_BUCKET", "bucket")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("GOOGLE_CLIENT_ID", "cid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_ALLOWED_DOMAIN", "iiitdmj.ac.in")
	t.Setenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback")
	t.Setenv("JWT_SECRET", "s3cr3t")
	t.Setenv("FRONTEND_URL", "http://localhost:5173")

	cfg, err := Load()

	require.NoError(t, err)
	require.Equal(t, "9090", cfg.Port)
	require.Equal(t, "iiitdmj.ac.in", cfg.GoogleAllowedDomain)
}

func TestLoad_MissingRequiredVar(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()

	require.Error(t, err)
}
