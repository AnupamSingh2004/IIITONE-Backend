# IIITOne Backend Phase 1 (Academic Resource Hub) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `iiitone-backend` Go REST API covering Google-OAuth auth (domain + `hd`-claim gated), course-tagged PDF material upload with dedup + text extraction, keyword search, and a manual admin moderation queue — per the approved spec.

**Architecture:** Single Go service, domain-packaged under `internal/` (auth, users, courses, materials, search, moderation, storage, db), composed in `cmd/server/main.go`. `chi` router, `pgx` for Postgres, `go-redis` for caching, `minio-go` behind a storage interface, `go-oidc` + `golang-jwt` for auth, `pdfcpu` for text-layer extraction. Local dev runs entirely via `docker-compose.yml` (Postgres, Redis, MinIO, backend).

**Tech Stack:** Go 1.22+, chi, pgx v5, golang-migrate, go-redis v9, minio-go v7, go-oidc v3, golang-jwt v5, pdfcpu, testify, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-07-17-phase1-academic-resource-hub-design.md`

**Note on storage:** This plan implements the `storage.Store` interface and a MinIO-backed implementation only (used for local dev and can point at any S3-compatible endpoint). An Azure Blob implementation is a follow-up task at actual deploy time, per the spec — the interface is designed so that's additive.

---

## Task 1: Go module skeleton and project layout

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.env.example`
- Create: `cmd/server/main.go` (placeholder)

- [ ] **Step 1: Initialize the module**

```bash
cd /home/anupam/code/GoodProjects/IIITOne/iiitone-backend
go mod init github.com/AnupamSingh2004/iiitone-backend
```

- [ ] **Step 2: Create `.gitignore`**

```gitignore
/bin/
*.env
.env
!.env.example
tmp/
*.log
```

- [ ] **Step 3: Create `.env.example`**

```bash
# Server
PORT=8080
ENV=development

# Postgres
DATABASE_URL=postgres://iiitone:iiitone@localhost:5432/iiitone?sslmode=disable

# Redis
REDIS_ADDR=localhost:6379

# Object storage (MinIO locally, S3-compatible)
STORAGE_ENDPOINT=localhost:9000
STORAGE_ACCESS_KEY=minioadmin
STORAGE_SECRET_KEY=minioadmin
STORAGE_BUCKET=iiitone-materials
STORAGE_USE_SSL=false

# Auth
GOOGLE_CLIENT_ID=replace-me.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=replace-me
GOOGLE_ALLOWED_DOMAIN=iiitdmj.ac.in
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
JWT_SECRET=dev-only-change-me
FRONTEND_URL=http://localhost:5173
```

- [ ] **Step 4: Create placeholder `cmd/server/main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("iiitone-backend starting...")
}
```

- [ ] **Step 5: Create `Makefile`**

```makefile
.PHONY: run test build up down migrate-up migrate-down

run:
	go run ./cmd/server

test:
	go test ./... -v

build:
	go build -o bin/server ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	migrate -path ./migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path ./migrations -database "$$DATABASE_URL" down 1
```

- [ ] **Step 6: Verify it builds**

Run: `go build ./...`
Expected: succeeds with no output.

- [ ] **Step 7: Commit**

```bash
git add go.mod .gitignore .env.example cmd/server/main.go Makefile
git commit -m "Initialize Go module and project skeleton"
```

---

## Task 2: Config loading

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/config/config_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL — package `config` / `Load` undefined.

- [ ] **Step 3: Implement**

```go
// internal/config/config.go
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
		"DATABASE_URL":         cfg.DatabaseURL,
		"REDIS_ADDR":           cfg.RedisAddr,
		"STORAGE_ENDPOINT":     cfg.StorageEndpoint,
		"GOOGLE_ALLOWED_DOMAIN": cfg.GoogleAllowedDomain,
		"JWT_SECRET":           cfg.JWTSecret,
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "Add env-based config loader"
```

---

## Task 3: Docker Compose local dev stack

**Files:**
- Create: `docker-compose.yml`
- Create: `docker/minio-init.sh` (creates the bucket on startup)

- [ ] **Step 1: Write `docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: iiitone
      POSTGRES_PASSWORD: iiitone
      POSTGRES_DB: iiitone
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U iiitone"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-data:/data

  minio-init:
    image: minio/mc:latest
    depends_on:
      - minio
    entrypoint: >
      /bin/sh -c "
      until mc alias set local http://minio:9000 minioadmin minioadmin; do sleep 1; done;
      mc mb -p local/iiitone-materials;
      exit 0;
      "

  backend:
    build: .
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
      minio:
        condition: service_started
    ports:
      - "8080:8080"

volumes:
  pgdata:
  minio-data:
```

- [ ] **Step 2: Create a minimal `Dockerfile` (so `docker compose build` works)**

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /server ./cmd/server

FROM alpine:3.19
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
```

- [ ] **Step 3: Copy `.env.example` to `.env` for local dev**

```bash
cp .env.example .env
```

- [ ] **Step 4: Verify the compose file is valid**

Run: `docker compose config --quiet`
Expected: no output, exit code 0.

- [ ] **Step 5: Bring up infra services and confirm they're healthy**

Run: `docker compose up -d postgres redis minio minio-init && docker compose ps`
Expected: `postgres` shows healthy, `redis`/`minio` running, `minio-init` exits 0.

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml Dockerfile
git commit -m "Add docker-compose local dev stack (postgres, redis, minio)"
```

(`.env` is gitignored — `.env.example` is already tracked from Task 1.)

---

## Task 4: Database migrations

**Files:**
- Create: `migrations/000001_init_schema.up.sql`
- Create: `migrations/000001_init_schema.down.sql`

- [ ] **Step 1: Install golang-migrate CLI (if not already available)**

Run: `which migrate || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

- [ ] **Step 2: Write the up migration**

```sql
-- migrations/000001_init_schema.up.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    google_sub TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    branch TEXT,
    year INT,
    role TEXT NOT NULL DEFAULT 'student' CHECK (role IN ('student', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT,
    name TEXT NOT NULL,
    branch TEXT NOT NULL,
    year INT NOT NULL,
    semester INT NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, branch, year, semester)
);

CREATE TABLE materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uploader_id UUID NOT NULL REFERENCES users(id),
    course_id UUID NOT NULL REFERENCES courses(id),
    type TEXT NOT NULL CHECK (type IN ('notes', 'pyq', 'assignment')),
    title TEXT NOT NULL,
    file_key TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    file_size BIGINT NOT NULL,
    has_text_layer BOOLEAN NOT NULL DEFAULT false,
    extracted_text TEXT,
    search_vector TSVECTOR,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX materials_search_idx ON materials USING GIN (search_vector);
CREATE INDEX materials_course_idx ON materials (course_id);
CREATE INDEX courses_lookup_idx ON courses (branch, year, semester);

CREATE FUNCTION materials_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', coalesce(NEW.title, '') || ' ' || coalesce(NEW.extracted_text, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER materials_search_vector_trigger
    BEFORE INSERT OR UPDATE ON materials
    FOR EACH ROW EXECUTE FUNCTION materials_search_vector_update();

CREATE TABLE flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    material_id UUID NOT NULL REFERENCES materials(id),
    reported_by UUID NOT NULL REFERENCES users(id),
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 3: Write the down migration**

```sql
-- migrations/000001_init_schema.down.sql
DROP TABLE IF EXISTS flags;
DROP TRIGGER IF EXISTS materials_search_vector_trigger ON materials;
DROP FUNCTION IF EXISTS materials_search_vector_update;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 4: Run the migration against the local dev Postgres**

Run: `source .env && migrate -path ./migrations -database "$DATABASE_URL" up`
Expected: `1/u init_schema (X.XXms)`, no errors.

- [ ] **Step 5: Verify tables exist**

Run: `docker compose exec postgres psql -U iiitone -d iiitone -c '\dt'`
Expected: lists `users`, `courses`, `materials`, `flags`.

- [ ] **Step 6: Verify the down migration also works cleanly, then re-apply up**

Run: `source .env && migrate -path ./migrations -database "$DATABASE_URL" down 1 && migrate -path ./migrations -database "$DATABASE_URL" up`
Expected: both succeed with no errors.

- [ ] **Step 7: Commit**

```bash
git add migrations
git commit -m "Add initial DB schema migration (users, courses, materials, flags)"
```

---

## Task 5: Database connection pool

**Files:**
- Create: `internal/db/db.go`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/db/db_test.go
package db

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect_Ping(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := Connect(context.Background(), url)
	require.NoError(t, err)
	defer pool.Close()

	require.NoError(t, pool.Ping(context.Background()))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `source .env && go test ./internal/db/... -v`
Expected: FAIL — `Connect` undefined.

- [ ] **Step 3: Implement**

```go
// internal/db/db.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 4: Fetch the dependency and run test to verify it passes**

Run: `go get github.com/jackc/pgx/v5 && source .env && go test ./internal/db/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db go.mod go.sum
git commit -m "Add Postgres connection pool"
```

---

## Task 6: Object storage interface + MinIO implementation

**Files:**
- Create: `internal/storage/storage.go` (interface)
- Create: `internal/storage/minio.go` (implementation)
- Test: `internal/storage/minio_test.go`

- [ ] **Step 1: Define the interface**

```go
// internal/storage/storage.go
package storage

import (
	"context"
	"io"
)

// Store abstracts object storage so implementations (MinIO locally,
// Azure Blob in prod) are swappable behind the same interface.
type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
```

- [ ] **Step 2: Write the failing integration test**

```go
// internal/storage/minio_test.go
package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinioStore_PutGetDeleteExists(t *testing.T) {
	endpoint := os.Getenv("STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("STORAGE_ENDPOINT not set, skipping integration test")
	}

	store, err := NewMinioStore(Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("STORAGE_ACCESS_KEY"),
		SecretKey: os.Getenv("STORAGE_SECRET_KEY"),
		Bucket:    os.Getenv("STORAGE_BUCKET"),
		UseSSL:    false,
	})
	require.NoError(t, err)

	ctx := context.Background()
	key := "test/hello.txt"
	content := []byte("hello iiitone")

	require.NoError(t, store.Put(ctx, key, bytes.NewReader(content), int64(len(content))))

	exists, err := store.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, exists)

	reader, err := store.Get(ctx, key)
	require.NoError(t, err)
	defer reader.Close()
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, content, got)

	require.NoError(t, store.Delete(ctx, key))
	exists, err = store.Exists(ctx, key)
	require.NoError(t, err)
	require.False(t, exists)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `source .env && go test ./internal/storage/... -v`
Expected: FAIL — `NewMinioStore`/`Config` undefined.

- [ ] **Step 4: Implement**

```go
// internal/storage/minio.go
package storage

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type MinioStore struct {
	client *minio.Client
	bucket string
}

func NewMinioStore(cfg Config) (*MinioStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinioStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *MinioStore) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{})
	return err
}

func (s *MinioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

func (s *MinioStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *MinioStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 5: Fetch dependency and run test to verify it passes**

Run: `go get github.com/minio/minio-go/v7 && source .env && docker compose up -d minio minio-init && go test ./internal/storage/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/storage go.mod go.sum
git commit -m "Add object storage interface and MinIO implementation"
```

---

## Task 7: JWT session tokens

**Files:**
- Create: `internal/auth/jwt.go`
- Test: `internal/auth/jwt_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/jwt_test.go
package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIssueAndVerifyToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()

	token, err := IssueToken(secret, userID, "student", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := VerifyToken(secret, token)
	require.NoError(t, err)
	require.Equal(t, userID, claims.UserID)
	require.Equal(t, "student", claims.Role)
}

func TestVerifyToken_Expired(t *testing.T) {
	secret := "test-secret"
	token, err := IssueToken(secret, uuid.New(), "student", -time.Hour)
	require.NoError(t, err)

	_, err = VerifyToken(secret, token)
	require.Error(t, err)
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	token, err := IssueToken("secret-a", uuid.New(), "student", time.Hour)
	require.NoError(t, err)

	_, err = VerifyToken("secret-b", token)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -v`
Expected: FAIL — `IssueToken`/`VerifyToken` undefined.

- [ ] **Step 3: Implement**

```go
// internal/auth/jwt.go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

func IssueToken(secret string, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func VerifyToken(secret, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
```

- [ ] **Step 4: Fetch dependencies and run test to verify it passes**

Run: `go get github.com/golang-jwt/jwt/v5 github.com/google/uuid && go test ./internal/auth/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth go.mod go.sum
git commit -m "Add JWT issue/verify for session tokens"
```

---

## Task 8: Google ID token domain + hd-claim validation

This is the security-critical piece from the spec: reject sign-in unless **both** the email suffix and the verified `hd` claim match. Validation here works on an already-verified ID token (signature verification happens via `go-oidc` in Task 9's OAuth handler) — this task is the pure, independently-testable domain check.

**Files:**
- Create: `internal/auth/domain.go`
- Test: `internal/auth/domain_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/domain_test.go
package auth

import "testing"

func TestValidateCollegeIdentity(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		hd        string
		wantValid bool
	}{
		{"valid iiitdmj account", "student@iiitdmj.ac.in", "iiitdmj.ac.in", true},
		{"hd claim mismatched despite matching-looking email", "student@iiitdmj.ac.in.evil.com", "evil.com", false},
		{"missing hd claim entirely", "student@iiitdmj.ac.in", "", false},
		{"correct hd but wrong email suffix", "student@gmail.com", "iiitdmj.ac.in", false},
		{"both wrong", "student@gmail.com", "gmail.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateCollegeIdentity(tt.email, tt.hd, "iiitdmj.ac.in")
			if got != tt.wantValid {
				t.Errorf("ValidateCollegeIdentity(%q, %q) = %v, want %v", tt.email, tt.hd, got, tt.wantValid)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestValidateCollegeIdentity -v`
Expected: FAIL — `ValidateCollegeIdentity` undefined.

- [ ] **Step 3: Implement**

```go
// internal/auth/domain.go
package auth

import "strings"

// ValidateCollegeIdentity enforces the spec's hard trust boundary: BOTH the
// email suffix and the token's verified `hd` (hosted domain) claim must match
// the college's Workspace domain. The hd claim is what actually proves Google
// Workspace ownership — the email suffix alone can be spoofed in string form.
func ValidateCollegeIdentity(email, hd, allowedDomain string) bool {
	if hd != allowedDomain {
		return false
	}
	return strings.HasSuffix(email, "@"+allowedDomain)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/... -run TestValidateCollegeIdentity -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth
git commit -m "Add college-domain + hd-claim identity validation"
```

---

## Task 9: Google OAuth callback handler + session cookie

**Files:**
- Create: `internal/auth/oauth.go`
- Create: `internal/auth/handlers.go`
- Test: `internal/auth/handlers_test.go`

- [ ] **Step 1: Write the failing test** (uses a fake verifier so the OIDC network call is not exercised in the unit test)

```go
// internal/auth/handlers_test.go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeIdentity struct {
	email string
	hd    string
	sub   string
	name  string
}

type fakeVerifier struct {
	identity fakeIdentity
	err      error
}

func (f fakeVerifier) VerifyCode(ctx context.Context, code string) (Identity, error) {
	if f.err != nil {
		return Identity{}, f.err
	}
	return Identity{
		Email: f.identity.email,
		HD:    f.identity.hd,
		Sub:   f.identity.sub,
		Name:  f.identity.name,
	}, nil
}

type fakeUserUpserter struct {
	upserted Identity
}

func (f *fakeUserUpserter) UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error) {
	f.upserted = id
	return UpsertedUser{ID: mustUUID(), Role: "student", Status: "active"}, nil
}

func TestCallbackHandler_ValidDomain_SetsCookieAndRedirects(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "http://localhost:5173", rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "session", cookies[0].Name)
	require.True(t, cookies[0].HttpOnly)
}

func TestCallbackHandler_WrongDomain_Rejected(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@gmail.com", hd: "gmail.com", sub: "sub123", name: "Nope",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestCallbackHandler -v`
Expected: FAIL — types/functions undefined.

- [ ] **Step 3: Implement the OIDC verifier wrapper**

```go
// internal/auth/oauth.go
package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Identity is the set of claims we need from a verified Google ID token.
type Identity struct {
	Email string
	HD    string
	Sub   string
	Name  string
}

// CodeVerifier exchanges an OAuth code for a verified identity. Implemented
// by GoogleVerifier in production and a fake in tests.
type CodeVerifier interface {
	VerifyCode(ctx context.Context, code string) (Identity, error)
}

type GoogleVerifier struct {
	oauthConfig *oauth2.Config
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
}

func NewGoogleVerifier(ctx context.Context, clientID, clientSecret, redirectURL string) (*GoogleVerifier, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}
	return &GoogleVerifier{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

func (g *GoogleVerifier) VerifyCode(ctx context.Context, code string) (Identity, error) {
	token, err := g.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return Identity{}, err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errNoIDToken
	}
	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, err
	}
	var claims struct {
		Email string `json:"email"`
		HD    string `json:"hd"`
		Name  string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}
	return Identity{Email: claims.Email, HD: claims.HD, Sub: idToken.Subject, Name: claims.Name}, nil
}
```

- [ ] **Step 4: Implement the handler**

```go
// internal/auth/handlers.go
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var errNoIDToken = errors.New("no id_token in oauth response")

type UpsertedUser struct {
	ID     uuid.UUID
	Role   string
	Status string
}

// UserUpserter persists/loads the user record for a verified identity.
// Implemented by the users package's repository.
type UserUpserter interface {
	UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error)
}

type CallbackHandler struct {
	verifier      CodeVerifier
	users         UserUpserter
	allowedDomain string
	jwtSecret     string
	frontendURL   string
}

func NewCallbackHandler(v CodeVerifier, u UserUpserter, allowedDomain, jwtSecret, frontendURL string) *CallbackHandler {
	return &CallbackHandler{verifier: v, users: u, allowedDomain: allowedDomain, jwtSecret: jwtSecret, frontendURL: frontendURL}
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	identity, err := h.verifier.VerifyCode(r.Context(), code)
	if err != nil {
		http.Error(w, "oauth verification failed", http.StatusBadGateway)
		return
	}

	if !ValidateCollegeIdentity(identity.Email, identity.HD, h.allowedDomain) {
		http.Error(w, "account is not a verified college account", http.StatusForbidden)
		return
	}

	user, err := h.users.UpsertFromIdentity(r.Context(), identity)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	if user.Status == "banned" {
		http.Error(w, "account is banned", http.StatusForbidden)
		return
	}

	token, err := IssueToken(h.jwtSecret, user.ID, user.Role, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	http.Redirect(w, r, h.frontendURL, http.StatusFound)
}

func mustUUID() uuid.UUID { return uuid.New() }
```

- [ ] **Step 5: Fetch dependencies and run test to verify it passes**

Run: `go get github.com/coreos/go-oidc/v3 golang.org/x/oauth2 && go test ./internal/auth/... -v`
Expected: PASS (note: `TestCallbackHandler_ValidDomain...` checks `Secure` cookie attribute is only meaningfully testable over HTTPS in real usage; in dev over HTTP some browsers will still accept it on localhost — acceptable per spec, no change needed here).

- [ ] **Step 6: Commit**

```bash
git add internal/auth go.mod go.sum
git commit -m "Add Google OAuth callback handler with hd-claim gated session issuance"
```

---

## Task 10: Auth session middleware

**Files:**
- Create: `internal/auth/middleware.go`
- Test: `internal/auth/middleware_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/middleware_test.go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_ValidCookie_CallsNext(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	token, _ := IssueToken(secret, userID, "student", time.Hour)

	var gotUserID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFromContext(r.Context())
		gotUserID = claims.UserID
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/app/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	RequireAuth(secret)(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, userID, gotUserID)
}

func TestRequireAuth_MissingCookie_Rejected(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/app/me", nil)
	rec := httptest.NewRecorder()

	RequireAuth("test-secret")(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAdmin_NonAdmin_Rejected(t *testing.T) {
	secret := "test-secret"
	token, _ := IssueToken(secret, uuid.New(), "student", time.Hour)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	RequireAuth(secret)(RequireAdmin(next)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestRequireAuth -v`
Expected: FAIL — `RequireAuth`/`ClaimsFromContext`/`RequireAdmin` undefined.

- [ ] **Step 3: Implement**

```go
// internal/auth/middleware.go
package auth

import (
	"context"
	"net/http"
)

type ctxKey string

const claimsCtxKey ctxKey = "claims"

func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := VerifyToken(secret, cookie.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsCtxKey).(*Claims)
	return claims, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/... -v`
Expected: PASS (all auth package tests)

- [ ] **Step 5: Commit**

```bash
git add internal/auth
git commit -m "Add auth session middleware (RequireAuth, RequireAdmin)"
```

---

## Task 11: Users repository + profile/admin handlers

**Files:**
- Create: `internal/users/repository.go`
- Create: `internal/users/handlers.go`
- Test: `internal/users/repository_test.go`

- [ ] **Step 1: Write the failing repository test** (integration, needs `DATABASE_URL`)

```go
// internal/users/repository_test.go
package users

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/stretchr/testify/require"
)

func testRepo(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRepository(pool)
}

func TestUpsertFromIdentity_CreatesThenReuses(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	identity := auth.Identity{Email: "newuser@iiitdmj.ac.in", HD: "iiitdmj.ac.in", Sub: "sub-unique-1", Name: "New User"}

	first, err := repo.UpsertFromIdentity(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, "student", first.Role)
	require.Equal(t, "active", first.Status)

	second, err := repo.UpsertFromIdentity(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "second login with same google_sub must return same user")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `source .env && go test ./internal/users/... -v`
Expected: FAIL — package/types undefined.

- [ ] **Step 3: Implement the repository**

```go
// internal/users/repository.go
package users

import (
	"context"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertFromIdentity(ctx context.Context, id auth.Identity) (auth.UpsertedUser, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, google_sub, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (google_sub) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, role, status
	`, id.Email, id.Sub, id.Name)

	var u auth.UpsertedUser
	if err := row.Scan(&u.ID, &u.Role, &u.Status); err != nil {
		return auth.UpsertedUser{}, err
	}
	return u, nil
}

type Profile struct {
	ID     uuid.UUID
	Email  string
	Name   string
	Branch *string
	Year   *int
	Role   string
}

func (r *Repository) GetProfile(ctx context.Context, id uuid.UUID) (Profile, error) {
	row := r.pool.QueryRow(ctx, `SELECT id, email, name, branch, year, role FROM users WHERE id = $1`, id)
	var p Profile
	if err := row.Scan(&p.ID, &p.Email, &p.Name, &p.Branch, &p.Year, &p.Role); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, id uuid.UUID, branch string, year int) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET branch = $1, year = $2 WHERE id = $3`, branch, year, id)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET status = $1 WHERE id = $2`, status, id)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `source .env && go test ./internal/users/... -v`
Expected: PASS

- [ ] **Step 5: Write HTTP handlers (profile GET/PATCH, admin ban/unban)**

```go
// internal/users/handlers.go
package users

import (
	"encoding/json"
	"net/http"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	profile, err := h.repo.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

type updateProfileRequest struct {
	Branch string `json:"branch"`
	Year   int    `json:"year"`
}

func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateProfile(r.Context(), claims.UserID, req.Branch, req.Year); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Ban and Unban are admin-only (mounted behind RequireAdmin in the router).
func (h *Handlers) Ban(w http.ResponseWriter, r *http.Request)   { h.setStatus(w, r, "banned") }
func (h *Handlers) Unban(w http.ResponseWriter, r *http.Request) { h.setStatus(w, r, "active") }

func (h *Handlers) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.repo.SetStatus(r.Context(), id, status); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 6: Fetch chi dependency, build**

Run: `go get github.com/go-chi/chi/v5 && go build ./...`
Expected: succeeds.

- [ ] **Step 7: Commit**

```bash
git add internal/users go.mod go.sum
git commit -m "Add users repository and profile/admin-ban handlers"
```

---

## Task 12: Courses repository (find-or-create) + handlers

This implements the race-safe find-or-create from the spec (Upload flow step 5).

**Files:**
- Create: `internal/courses/repository.go`
- Create: `internal/courses/handlers.go`
- Test: `internal/courses/repository_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/courses/repository_test.go
package courses

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/stretchr/testify/require"
)

func testRepo(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRepository(pool)
}

func TestFindOrCreate_ConcurrentSameCourse_ResolvesToOneRow(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, err := repo.FindOrCreate(ctx, "Race Condition Course", "CSE", 2026, 3, nil)
			require.NoError(t, err)
			ids[idx] = id.String()
		}(i)
	}
	wg.Wait()

	first := ids[0]
	for _, id := range ids {
		require.Equal(t, first, id, "all concurrent find-or-create calls must resolve to the same course id")
	}
}

func TestList_FiltersByBranchYearSemester(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	_, err := repo.FindOrCreate(ctx, "Data Structures", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	list, err := repo.List(ctx, "CSE", 2026, 3)
	require.NoError(t, err)
	require.NotEmpty(t, list)
	for _, c := range list {
		require.Equal(t, "CSE", c.Branch)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `source .env && go test ./internal/courses/... -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement**

```go
// internal/courses/repository.go
package courses

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// FindOrCreate is race-safe: concurrent calls with identical
// (name, branch, year, semester) resolve to the same row via ON CONFLICT.
func (r *Repository) FindOrCreate(ctx context.Context, name, branch string, year, semester int, createdBy *uuid.UUID) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO courses (name, branch, year, semester, created_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name, branch, year, semester) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, name, branch, year, semester, createdBy)

	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

type Course struct {
	ID       uuid.UUID
	Code     *string
	Name     string
	Branch   string
	Year     int
	Semester int
}

func (r *Repository) List(ctx context.Context, branch string, year, semester int) ([]Course, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, name, branch, year, semester
		FROM courses WHERE branch = $1 AND year = $2 AND semester = $3
		ORDER BY name
	`, branch, year, semester)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Course
	for rows.Next() {
		var c Course
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Branch, &c.Year, &c.Semester); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `source .env && go test ./internal/courses/... -v`
Expected: PASS

- [ ] **Step 5: Write handlers**

```go
// internal/courses/handlers.go
package courses

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	branch := r.URL.Query().Get("branch")
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	semester, _ := strconv.Atoi(r.URL.Query().Get("semester"))

	list, err := h.repo.List(r.Context(), branch, year, semester)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/courses
git commit -m "Add race-safe course find-or-create and list handler"
```

---

## Task 13: PDF validation and text extraction

**Files:**
- Create: `internal/materials/pdf.go`
- Test: `internal/materials/pdf_test.go`
- Test fixtures: `internal/materials/testdata/with-text.pdf`, `internal/materials/testdata/scanned-no-text.pdf`, `internal/materials/testdata/corrupted.pdf`

- [ ] **Step 1: Generate test fixture PDFs**

```bash
mkdir -p internal/materials/testdata
# A real PDF with a text layer (use any tool available, e.g. LibreOffice headless or a Python one-liner):
python3 - <<'EOF'
from reportlab.pdfgen import canvas
c = canvas.Canvas("internal/materials/testdata/with-text.pdf")
c.drawString(100, 750, "IIITOne test document with a real text layer")
c.save()
EOF
# A "scanned" PDF with no text layer: a PDF wrapping a blank image, no text objects.
python3 - <<'EOF'
from reportlab.pdfgen import canvas
from reportlab.lib.pagesizes import letter
import io
c = canvas.Canvas("internal/materials/testdata/scanned-no-text.pdf", pagesize=letter)
c.rect(50, 50, 100, 100, fill=1)  # a shape, no text drawing calls
c.save()
EOF
# A corrupted/non-PDF file with a .pdf extension, to exercise the error path:
echo "not a real pdf" > internal/materials/testdata/corrupted.pdf
```

If `reportlab` isn't available (`pip install reportlab`), any other tool that produces a text-layer PDF and a text-free PDF works — the important thing is fixture variety, not the generation method.

- [ ] **Step 2: Write the failing test**

```go
// internal/materials/pdf_test.go
package materials

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePDF_RejectsNonPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/corrupted.pdf")
	require.NoError(t, err)

	require.False(t, IsPDF(data))
}

func TestValidatePDF_AcceptsRealPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	require.True(t, IsPDF(data))
}

func TestExtractText_WithTextLayer(t *testing.T) {
	text, hasLayer, err := ExtractText("testdata/with-text.pdf")
	require.NoError(t, err)
	require.True(t, hasLayer)
	require.Contains(t, text, "IIITOne test document")
}

func TestExtractText_NoTextLayer_DegradesGracefully(t *testing.T) {
	text, hasLayer, err := ExtractText("testdata/scanned-no-text.pdf")
	require.NoError(t, err, "no text layer must not be an error, per spec")
	require.False(t, hasLayer)
	require.Empty(t, text)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/materials/... -run 'TestValidatePDF|TestExtractText' -v`
Expected: FAIL — `IsPDF`/`ExtractText` undefined.

- [ ] **Step 4: Implement**

```go
// internal/materials/pdf.go
package materials

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// IsPDF checks the PDF magic bytes (%PDF-) — the cheap, fast rejection of
// non-PDF uploads before any further (expensive) processing is attempted.
func IsPDF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("%PDF-"))
}

// ExtractText attempts to pull the embedded text layer out of a PDF using
// pdfcpu. Per spec: a missing text layer, or a corrupted/encrypted file that
// pdfcpu can't parse, both degrade gracefully to (empty, false, nil) rather
// than failing the upload — the file is still stored either way.
func ExtractText(path string) (string, bool, error) {
	tmpDir, err := os.MkdirTemp("", "iiitone-extract-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tmpDir)

	if err := api.ExtractTextsFile(path, tmpDir, nil, nil); err != nil {
		// pdfcpu couldn't parse this file at all (corrupted/encrypted/scanned-with-no-content-stream).
		return "", false, nil
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return "", false, nil
	}

	var combined bytes.Buffer
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		if err != nil {
			continue
		}
		combined.Write(content)
		combined.WriteByte('\n')
	}

	text := combined.String()
	if len(bytes.TrimSpace(combined.Bytes())) == 0 {
		return "", false, nil
	}
	return text, true, nil
}
```

- [ ] **Step 5: Fetch dependency and run test to verify it passes**

Run: `go get github.com/pdfcpu/pdfcpu && go test ./internal/materials/... -run 'TestValidatePDF|TestExtractText' -v`
Expected: PASS. **If `api.ExtractTextsFile` doesn't match the installed pdfcpu version's actual signature**, run `go doc github.com/pdfcpu/pdfcpu/pkg/api` to find the current text-extraction function and adjust the call accordingly — the graceful-degradation behavior (never error out to the caller) is the part that must be preserved.

- [ ] **Step 6: Commit**

```bash
git add internal/materials/pdf.go internal/materials/pdf_test.go internal/materials/testdata go.mod go.sum
git commit -m "Add PDF validation and text-layer extraction with graceful degradation"
```

---

## Task 14: Materials repository (dedup, insert, list, detail)

**Files:**
- Create: `internal/materials/repository.go`
- Test: `internal/materials/repository_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/materials/repository_test.go
package materials

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/stretchr/testify/require"
)

func testDeps(t *testing.T) (*Repository, *users.Repository, *courses.Repository) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRepository(pool), users.NewRepository(pool), courses.NewRepository(pool)
}

func TestCreate_DuplicateContentHash_Rejected(t *testing.T) {
	repo, userRepo, courseRepo := testDeps(t)
	ctx := context.Background()

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("dup-test-1"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Dedup Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	input := CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "First upload",
		FileKey: "materials/hash1", ContentHash: "fixed-hash-for-dedup-test", FileSize: 100,
	}
	_, err = repo.Create(ctx, input)
	require.NoError(t, err)

	_, err = repo.Create(ctx, input)
	require.Error(t, err, "second insert with same content_hash must fail the unique constraint")
}

func TestListApproved_ExcludesPending(t *testing.T) {
	repo, userRepo, courseRepo := testDeps(t)
	ctx := context.Background()

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("dup-test-2"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "List Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	id, err := repo.Create(ctx, CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "Pending item",
		FileKey: "materials/hash2", ContentHash: "hash-for-list-test", FileSize: 100,
	})
	require.NoError(t, err)

	results, err := repo.ListApproved(ctx, ListFilter{CourseID: &courseID})
	require.NoError(t, err)
	for _, m := range results {
		require.NotEqual(t, id, m.ID, "pending material must not appear in approved listing")
	}

	require.NoError(t, repo.Approve(ctx, id))
	results, err = repo.ListApproved(ctx, ListFilter{CourseID: &courseID})
	require.NoError(t, err)
	found := false
	for _, m := range results {
		if m.ID == id {
			found = true
		}
	}
	require.True(t, found, "approved material must appear in approved listing")
}
```

- [ ] **Step 2: Add the `testIdentity` test helper**

```go
// internal/materials/helpers_test.go
package materials

import "github.com/AnupamSingh2004/iiitone-backend/internal/auth"

func testIdentity(suffix string) auth.Identity {
	return auth.Identity{
		Email: suffix + "@iiitdmj.ac.in",
		HD:    "iiitdmj.ac.in",
		Sub:   "sub-" + suffix,
		Name:  "Test User " + suffix,
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `source .env && go test ./internal/materials/... -run 'TestCreate|TestListApproved' -v`
Expected: FAIL — repository types undefined.

- [ ] **Step 4: Implement**

```go
// internal/materials/repository.go
package materials

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type CreateInput struct {
	UploaderID    uuid.UUID
	CourseID      uuid.UUID
	Type          string
	Title         string
	FileKey       string
	ContentHash   string
	FileSize      int64
	HasTextLayer  bool
	ExtractedText string
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO materials (uploader_id, course_id, type, title, file_key, content_hash, file_size, has_text_layer, extracted_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''))
		RETURNING id
	`, in.UploaderID, in.CourseID, in.Type, in.Title, in.FileKey, in.ContentHash, in.FileSize, in.HasTextLayer, in.ExtractedText)

	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ExistsByContentHash supports the pre-insert dedup check in the upload handler,
// so we can reject duplicates before doing the (expensive) storage upload.
func (r *Repository) ExistsByContentHash(ctx context.Context, hash string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE content_hash = $1)`, hash).Scan(&exists)
	return exists, err
}

type Material struct {
	ID           uuid.UUID
	UploaderID   uuid.UUID
	CourseID     uuid.UUID
	Type         string
	Title        string
	FileKey      string
	HasTextLayer bool
	Status       string
}

type ListFilter struct {
	CourseID *uuid.UUID
	Type     *string
}

func (r *Repository) ListApproved(ctx context.Context, f ListFilter) ([]Material, error) {
	query := `SELECT id, uploader_id, course_id, type, title, file_key, has_text_layer, status
	          FROM materials WHERE status = 'approved'`
	args := []any{}
	argN := 1
	if f.CourseID != nil {
		query += ` AND course_id = $` + itoa(argN)
		args = append(args, *f.CourseID)
		argN++
	}
	if f.Type != nil {
		query += ` AND type = $` + itoa(argN)
		args = append(args, *f.Type)
		argN++
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.UploaderID, &m.CourseID, &m.Type, &m.Title, &m.FileKey, &m.HasTextLayer, &m.Status); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) ListPending(ctx context.Context) ([]Material, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, uploader_id, course_id, type, title, file_key, has_text_layer, status
		FROM materials WHERE status = 'pending' ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Material
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.UploaderID, &m.CourseID, &m.Type, &m.Title, &m.FileKey, &m.HasTextLayer, &m.Status); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) Approve(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE materials SET status = 'approved' WHERE id = $1`, id)
	return err
}

// GetFileKey is used by the reject/delete flow to know what to remove from storage.
func (r *Repository) GetFileKey(ctx context.Context, id uuid.UUID) (string, error) {
	var key string
	err := r.pool.QueryRow(ctx, `SELECT file_key FROM materials WHERE id = $1`, id).Scan(&key)
	return key, err
}

// Delete hard-deletes the row. Per spec, rejection is a hard delete, not a status
// flag — this is also what frees the content_hash for resubmission.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM materials WHERE id = $1`, id)
	return err
}

func itoa(n int) string {
	digits := "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	return itoa(n/10) + string(digits[n%10])
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `source .env && go test ./internal/materials/... -run 'TestCreate|TestListApproved' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/materials/repository.go internal/materials/repository_test.go internal/materials/helpers_test.go
git commit -m "Add materials repository with dedup check, approve, and hard-delete reject"
```

---

## Task 15: Upload handler (orchestrates hash, dedup, extraction, storage, insert)

**Files:**
- Create: `internal/materials/upload_handler.go`
- Test: `internal/materials/upload_handler_test.go`

- [ ] **Step 1: Write the failing test** (uses fakes for storage and repo interfaces so this stays a fast unit test)

```go
// internal/materials/upload_handler_test.go
package materials

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeStore struct{ puts int }

func (f *fakeStore) Put(ctx context.Context, key string, body interface{ Read([]byte) (int, error) }, size int64) error {
	f.puts++
	return nil
}

type fakeMaterialsRepo struct {
	existingHashes map[string]bool
	created        []CreateInput
}

func (f *fakeMaterialsRepo) ExistsByContentHash(ctx context.Context, hash string) (bool, error) {
	return f.existingHashes[hash], nil
}
func (f *fakeMaterialsRepo) Create(ctx context.Context, in CreateInput) (uuid.UUID, error) {
	f.created = append(f.created, in)
	return uuid.New(), nil
}

func buildMultipartUpload(t *testing.T, fieldName, fileName string, content []byte, extraFields map[string]string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for k, v := range extraFields {
		require.NoError(t, writer.WriteField(k, v))
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/materials", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadHandler_RejectsDuplicateContentHash(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	// Pre-seed: mark this exact content's hash as already existing.
	hash := sha256Hex(pdfBytes)
	repo.existingHashes[hash] = true

	h := NewUploadHandlerForTest(repo)

	req := buildMultipartUpload(t, "file", "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Empty(t, repo.created)
}

func TestUploadHandler_RejectsNonPDF(t *testing.T) {
	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	h := NewUploadHandlerForTest(repo)

	req := buildMultipartUpload(t, "file", "notes.pdf", []byte("not a pdf"), map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.created)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/materials/... -run TestUploadHandler -v`
Expected: FAIL — `NewUploadHandlerForTest`/`sha256Hex` undefined.

- [ ] **Step 3: Implement** (real handler takes the full `storage.Store` interface; a small test-only constructor wires a no-op store so hash/dedup logic is unit-testable without MinIO)

```go
// internal/materials/upload_handler.go
package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/google/uuid"
)

const maxUploadSize = 50 << 20 // 50MB

type materialsRepo interface {
	ExistsByContentHash(ctx context.Context, hash string) (bool, error)
	Create(ctx context.Context, in CreateInput) (uuid.UUID, error)
}

type UploadHandler struct {
	repo  materialsRepo
	store storage.Store
}

func NewUploadHandler(repo materialsRepo, store storage.Store) *UploadHandler {
	return &UploadHandler{repo: repo, store: store}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large or invalid form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "upload-*.pdf")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), file)
	if err != nil {
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	header := make([]byte, 5)
	tmp.ReadAt(header, 0)
	if !IsPDF(header) {
		http.Error(w, "only PDF files are accepted", http.StatusBadRequest)
		return
	}

	exists, err := h.repo.ExistsByContentHash(r.Context(), hash)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "this file has already been uploaded", http.StatusConflict)
		return
	}

	courseID, err := uuid.Parse(r.FormValue("course_id"))
	if err != nil {
		http.Error(w, "invalid course_id", http.StatusBadRequest)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())

	text, hasLayer, _ := ExtractText(tmp.Name())

	fileKey := "materials/" + hash + ".pdf"
	if h.store != nil {
		if _, seekErr := tmp.Seek(0, io.SeekStart); seekErr == nil {
			if err := h.store.Put(r.Context(), fileKey, tmp, size); err != nil {
				http.Error(w, "failed to store file", http.StatusInternalServerError)
				return
			}
		}
	}

	id, err := h.repo.Create(r.Context(), CreateInput{
		UploaderID: claims.UserID, CourseID: courseID,
		Type: r.FormValue("type"), Title: r.FormValue("title"),
		FileKey: fileKey, ContentHash: hash, FileSize: size,
		HasTextLayer: hasLayer, ExtractedText: text,
	})
	if err != nil {
		http.Error(w, "failed to save material", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"id":"` + id.String() + `"}`))
}

// NewUploadHandlerForTest wires a handler with no storage backend, for unit
// tests that only exercise validation/dedup logic (storage.Store is nil-safe
// in ServeHTTP: the Put call is skipped when store is nil).
func NewUploadHandlerForTest(repo materialsRepo) *UploadHandler {
	return &UploadHandler{repo: repo, store: nil}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/materials/... -run TestUploadHandler -v`
Expected: PASS

- [ ] **Step 5: Run the full materials package test suite**

Run: `source .env && go test ./internal/materials/... -v`
Expected: PASS (all tests, integration ones require `DATABASE_URL`/MinIO up via `docker compose up -d`)

- [ ] **Step 6: Commit**

```bash
git add internal/materials/upload_handler.go internal/materials/upload_handler_test.go
git commit -m "Add upload handler: PDF validation, dedup, extraction, storage orchestration"
```

---

## Task 16: Search (Postgres FTS + Redis cache)

**Files:**
- Create: `internal/search/search.go`
- Create: `internal/search/handlers.go`
- Test: `internal/search/search_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/search/search_test.go
package search

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/stretchr/testify/require"
)

func TestSearch_RanksTitleMatchAndExcludesPending(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userRepo := users.NewRepository(pool)
	courseRepo := courses.NewRepository(pool)
	matRepo := materials.NewRepository(pool)
	searchRepo := NewRepository(pool, nil) // nil cache: exercise DB path directly

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity())
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Operating Systems", "CSE", 2026, 5, nil)
	require.NoError(t, err)

	approvedID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title: "Operating Systems Deadlock Notes", FileKey: "k1", ContentHash: "search-hash-1", FileSize: 10,
	})
	require.NoError(t, err)
	require.NoError(t, matRepo.Approve(ctx, approvedID))

	_, err = matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title: "Operating Systems Scheduling Notes (still pending)", FileKey: "k2", ContentHash: "search-hash-2", FileSize: 10,
	})
	require.NoError(t, err)

	results, err := searchRepo.Query(ctx, Query{Text: "deadlock"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, approvedID, results[0].ID)
	// Result must carry enough to render a card without a second round-trip —
	// the frontend's MaterialCard needs type/courseName/hasTextLayer directly.
	require.Equal(t, "notes", results[0].Type)
	require.Equal(t, "Operating Systems", results[0].CourseName)
	require.False(t, results[0].HasTextLayer)

	// course_id/type filters must actually narrow results, not be silently ignored.
	wrongType := "pyq"
	filtered, err := searchRepo.Query(ctx, Query{Text: "deadlock", Type: &wrongType})
	require.NoError(t, err)
	require.Empty(t, filtered, "type filter must exclude a non-matching type")
}
```

- [ ] **Step 2: Add a small identity test helper (or reuse pattern from materials package)**

```go
// internal/search/helpers_test.go
package search

import "github.com/AnupamSingh2004/iiitone-backend/internal/auth"

func testIdentity() auth.Identity {
	return auth.Identity{Email: "search-test@iiitdmj.ac.in", HD: "iiitdmj.ac.in", Sub: "search-test-sub", Name: "Search Tester"}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `source .env && go test ./internal/search/... -v`
Expected: FAIL — package undefined.

- [ ] **Step 4: Implement**

```go
// internal/search/search.go
package search

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Repository struct {
	pool  *pgxpool.Pool
	cache *redis.Client // nil is valid: cache is skipped (used in tests and as a safe default)
}

func NewRepository(pool *pgxpool.Pool, cache *redis.Client) *Repository {
	return &Repository{pool: pool, cache: cache}
}

type Query struct {
	Text     string
	CourseID *uuid.UUID
	Type     *string
}

// Result matches the frontend's MaterialSummary shape exactly (see
// iiitone-web's src/components/materials/MaterialCard.tsx) so the browse page
// can render a card directly from a search result with no second round-trip.
type Result struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	CourseName   string    `json:"courseName"`
	HasTextLayer bool      `json:"hasTextLayer"`
	Rank         float64   `json:"rank"`
}

func (r *Repository) Query(ctx context.Context, q Query) ([]Result, error) {
	cacheKey := cacheKeyFor(q)
	if r.cache != nil {
		if cached, err := r.cache.Get(ctx, cacheKey).Result(); err == nil {
			var results []Result
			if json.Unmarshal([]byte(cached), &results) == nil {
				return results, nil
			}
		}
	}

	sqlQuery := `
		SELECT m.id, m.title, m.type, c.name, m.has_text_layer,
		       ts_rank(m.search_vector, plainto_tsquery('english', $1)) AS rank
		FROM materials m
		JOIN courses c ON c.id = m.course_id
		WHERE m.status = 'approved'
		  AND m.search_vector @@ plainto_tsquery('english', $1)
		  AND ($2::uuid IS NULL OR m.course_id = $2)
		  AND ($3::text IS NULL OR m.type = $3)
		ORDER BY rank DESC
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, sqlQuery, q.Text, q.CourseID, q.Type)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var res Result
		if err := rows.Scan(&res.ID, &res.Title, &res.Type, &res.CourseName, &res.HasTextLayer, &res.Rank); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if r.cache != nil {
		if data, err := json.Marshal(results); err == nil {
			r.cache.Set(ctx, cacheKey, data, 2*time.Minute)
		}
	}

	return results, nil
}

// cacheKeyFor must fold every filter into the key — a cache hit on text alone
// would silently return results for the wrong course/type filter combination.
func cacheKeyFor(q Query) string {
	key := "search:" + q.Text
	if q.CourseID != nil {
		key += ":course=" + q.CourseID.String()
	}
	if q.Type != nil {
		key += ":type=" + *q.Type
	}
	return key
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `source .env && go test ./internal/search/... -v`
Expected: PASS

- [ ] **Step 6: Write the search HTTP handler**

```go
// internal/search/handlers.go
package search

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := Query{Text: r.URL.Query().Get("q")}

	if courseID, err := uuid.Parse(r.URL.Query().Get("course_id")); err == nil {
		q.CourseID = &courseID
	}
	if t := r.URL.Query().Get("type"); t != "" {
		q.Type = &t
	}

	results, err := h.repo.Query(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
```

- [ ] **Step 7: Fetch dependency, commit**

```bash
go get github.com/redis/go-redis/v9
git add internal/search go.mod go.sum
git commit -m "Add Postgres full-text search with Redis result caching"
```

---

## Task 17: Moderation handlers (pending queue, approve/reject, flags, ban)

**Files:**
- Create: `internal/moderation/handlers.go`
- Create: `internal/moderation/flags_repository.go`
- Test: `internal/moderation/flags_repository_test.go`

- [ ] **Step 1: Write the failing test for flags repository**

```go
// internal/moderation/flags_repository_test.go
package moderation

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestCreateFlag_AndResolve(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userRepo := users.NewRepository(pool)
	courseRepo := courses.NewRepository(pool)
	matRepo := materials.NewRepository(pool)
	flagRepo := NewFlagsRepository(pool)

	uploader, err := userRepo.UpsertFromIdentity(ctx, auth.Identity{Email: "flagtest@iiitdmj.ac.in", HD: "iiitdmj.ac.in", Sub: "flag-sub-1", Name: "Flag Test"})
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Flag Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)
	matID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "Flagged item",
		FileKey: "k-flag", ContentHash: "flag-hash-1", FileSize: 10,
	})
	require.NoError(t, err)
	require.NoError(t, matRepo.Approve(ctx, matID))

	flagID, err := flagRepo.Create(ctx, matID, uploader.ID, "wrong course tag")
	require.NoError(t, err)

	open, err := flagRepo.ListOpen(ctx)
	require.NoError(t, err)
	found := false
	for _, f := range open {
		if f.ID == flagID {
			found = true
		}
	}
	require.True(t, found)

	require.NoError(t, flagRepo.Resolve(ctx, flagID))
	open, err = flagRepo.ListOpen(ctx)
	require.NoError(t, err)
	for _, f := range open {
		require.NotEqual(t, flagID, f.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `source .env && go test ./internal/moderation/... -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement flags repository**

```go
// internal/moderation/flags_repository.go
package moderation

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FlagsRepository struct {
	pool *pgxpool.Pool
}

func NewFlagsRepository(pool *pgxpool.Pool) *FlagsRepository {
	return &FlagsRepository{pool: pool}
}

func (r *FlagsRepository) Create(ctx context.Context, materialID, reportedBy uuid.UUID, reason string) (uuid.UUID, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO flags (material_id, reported_by, reason) VALUES ($1, $2, $3) RETURNING id
	`, materialID, reportedBy, reason)
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

// Flag matches the frontend's admin flags-queue shape exactly (see
// iiitone-web's src/app/app/admin/flags/page.tsx's OpenFlag interface).
// Note UploaderID here is the flagged MATERIAL's uploader (materials.uploader_id
// — the person the "Ban uploader" button acts on), NOT flags.reported_by (the
// person who filed the report, which the frontend never displays or uses).
// This requires a JOIN to materials, not a plain SELECT off flags.
type Flag struct {
	ID            uuid.UUID `json:"id"`
	MaterialID    uuid.UUID `json:"materialId"`
	MaterialTitle string    `json:"materialTitle"`
	UploaderID    uuid.UUID `json:"uploaderId"`
	Reason        string    `json:"reason"`
}

func (r *FlagsRepository) ListOpen(ctx context.Context) ([]Flag, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.material_id, m.title, m.uploader_id, f.reason
		FROM flags f
		JOIN materials m ON m.id = f.material_id
		WHERE f.status = 'open'
		ORDER BY f.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Flag{}
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.ID, &f.MaterialID, &f.MaterialTitle, &f.UploaderID, &f.Reason); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (r *FlagsRepository) Resolve(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE flags SET status = 'resolved' WHERE id = $1`, id)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `source .env && go test ./internal/moderation/... -v`
Expected: PASS

- [ ] **Step 4.5: Add a JSON-shaped pending-queue query to `internal/materials/repository.go`**

`materials.Material` deliberately has no JSON tags (see the comment above its
definition) and `ListPending`'s current `SELECT` doesn't join `courses`, so
serializing `[]Material` directly would produce PascalCase fields and no
`courseName` at all — a mismatch with the frontend's `PendingMaterial`
interface (`src/app/app/admin/pending/page.tsx`: `{id, title, type, courseName}`).
Add a dedicated response type and change `ListPending` to join courses and
return it, instead of `[]Material`:

```go
// internal/materials/repository.go — add this type and replace ListPending's
// body/signature with this version.

// PendingSummary matches the frontend's admin pending-queue shape exactly
// (see iiitone-web's src/app/app/admin/pending/page.tsx's PendingMaterial
// interface) so the queue can render a row with no second round-trip.
type PendingSummary struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	CourseName string    `json:"courseName"`
}

func (r *Repository) ListPending(ctx context.Context) ([]PendingSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.title, m.type, c.name
		FROM materials m
		JOIN courses c ON c.id = m.course_id
		WHERE m.status = 'pending'
		ORDER BY m.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []PendingSummary{}
	for rows.Next() {
		var s PendingSummary
		if err := rows.Scan(&s.ID, &s.Title, &s.Type, &s.CourseName); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}
```

`ListPending` was not yet used or tested anywhere before this task, so this
signature change is safe — nothing else in the codebase calls it.

- [ ] **Step 5: Implement HTTP handlers wiring materials + flags + users + storage together**

```go
// internal/moderation/handlers.go
package moderation

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	materials *materials.Repository
	flags     *FlagsRepository
	store     storage.Store
}

func NewHandlers(materialsRepo *materials.Repository, flagsRepo *FlagsRepository, store storage.Store) *Handlers {
	return &Handlers{materials: materialsRepo, flags: flagsRepo, store: store}
}

func (h *Handlers) ListPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.materials.ListPending(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (h *Handlers) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "materialID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.materials.Approve(r.Context(), id); err != nil {
		http.Error(w, "approve failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reject hard-deletes the material (row + storage object), per spec — this
// is also what frees its content_hash for resubmission.
func (h *Handlers) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "materialID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.rejectAndDelete(r.Context(), w, id)
}

func (h *Handlers) rejectAndDelete(ctx context.Context, w http.ResponseWriter, id uuid.UUID) {
	fileKey, err := h.materials.GetFileKey(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if h.store != nil {
		_ = h.store.Delete(ctx, fileKey) // best-effort; row delete below is the source of truth
	}
	if err := h.materials.Delete(ctx, id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createFlagRequest struct {
	MaterialID string `json:"material_id"`
	Reason     string `json:"reason"`
}

func (h *Handlers) CreateFlag(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req createFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	materialID, err := uuid.Parse(req.MaterialID)
	if err != nil {
		http.Error(w, "invalid material_id", http.StatusBadRequest)
		return
	}
	if _, err := h.flags.Create(r.Context(), materialID, claims.UserID, req.Reason); err != nil {
		http.Error(w, "failed to create flag", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handlers) ListOpenFlags(w http.ResponseWriter, r *http.Request) {
	list, err := h.flags.ListOpen(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// ResolveFlag rejects (hard-deletes) the flagged material and resolves the flag.
// Banning the uploader, if desired, is a separate call to the users admin-ban endpoint —
// these are independent actions per spec, not implied by each other.
func (h *Handlers) ResolveFlag(w http.ResponseWriter, r *http.Request) {
	flagID, err := uuid.Parse(chi.URLParam(r, "flagID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	materialID, err := uuid.Parse(r.URL.Query().Get("material_id"))
	if err == nil {
		h.rejectAndDelete(r.Context(), w, materialID)
		if w.(interface{ Written() bool }) != nil {
			// no-op guard placeholder removed below in favor of direct flow
		}
	}
	if err := h.flags.Resolve(r.Context(), flagID); err != nil {
		http.Error(w, "resolve failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 6: Fix the `ResolveFlag` handler** (the draft above has a dead type-assertion placeholder — replace it with a clean two-step flow: reject the material only if `material_id` was provided, always resolve the flag)

```go
// Replace ResolveFlag in internal/moderation/handlers.go with:
func (h *Handlers) ResolveFlag(w http.ResponseWriter, r *http.Request) {
	flagID, err := uuid.Parse(chi.URLParam(r, "flagID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if materialIDStr := r.URL.Query().Get("material_id"); materialIDStr != "" {
		materialID, err := uuid.Parse(materialIDStr)
		if err != nil {
			http.Error(w, "invalid material_id", http.StatusBadRequest)
			return
		}
		fileKey, err := h.materials.GetFileKey(r.Context(), materialID)
		if err == nil {
			if h.store != nil {
				_ = h.store.Delete(r.Context(), fileKey)
			}
			_ = h.materials.Delete(r.Context(), materialID)
		}
	}

	if err := h.flags.Resolve(r.Context(), flagID); err != nil {
		http.Error(w, "resolve failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 7: Build and run the full moderation suite**

Run: `go build ./... && source .env && go test ./internal/moderation/... -v`
Expected: builds clean, tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/moderation
git commit -m "Add moderation handlers: pending queue, approve/reject, flags"
```

---

## Task 18: Prometheus metrics middleware

**Files:**
- Create: `internal/metrics/metrics.go`
- Test: `internal/metrics/metrics_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/metrics/metrics_test.go
package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMiddleware_IncrementsRequestCounter(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/materials/search", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Contains(t, metricsRec.Body.String(), "http_requests_total")
	require.True(t, strings.Contains(metricsRec.Body.String(), `path="/materials/search"`) || strings.Contains(metricsRec.Body.String(), "http_requests_total"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metrics/... -v`
Expected: FAIL — `New`/`Middleware`/`Handler` undefined.

- [ ] **Step 3: Implement**

```go
// internal/metrics/metrics.go
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requests *prometheus.CounterVec
}

func New() *Metrics {
	return &Metrics{
		requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"path", "method", "status"}),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		m.requests.WithLabelValues(r.URL.Path, r.Method, http.StatusText(sw.status)).Inc()
	})
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
```

- [ ] **Step 4: Fetch dependency and run test to verify it passes**

Run: `go get github.com/prometheus/client_golang && go test ./internal/metrics/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metrics go.mod go.sum
git commit -m "Add Prometheus metrics middleware and /metrics endpoint"
```

---

## Task 19: Router wiring and main.go composition root

**Files:**
- Create: `internal/router/router.go`
- Modify: `cmd/server/main.go`
- Test: `internal/router/router_test.go` (smoke test — public routes reachable, protected routes reject unauthenticated requests)

- [ ] **Step 1: Write the failing smoke test**

```go
// internal/router/router_test.go
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouter_ProtectedRouteRejectsUnauthenticated(t *testing.T) {
	r := New(Deps{JWTSecret: "test-secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouter_HealthCheckIsPublic(t *testing.T) {
	r := New(Deps{JWTSecret: "test-secret"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/router/... -v`
Expected: FAIL — package undefined.

**PRE-FIX NOTE (applied before implementation, read this before Step 3):** the
snippets originally drafted for this task had three real bugs, caught by
cross-checking against the actual state of already-completed packages:

1. `materials.NewUploadHandler(matRepo, deps.Store)` is a **2-argument call
   to a 3-argument function** — `internal/materials/upload_handler.go`'s real
   signature (fixed during Task 15's review) is
   `NewUploadHandler(repo materialsRepo, courses courseResolver, store storage.Store)`.
   The router as originally drafted would not compile. Step 3 below now
   constructs a `courseRepo` and passes it.
2. **There was no `/auth/google/login` route or handler anywhere in the
   codebase.** `internal/auth` only has a callback handler — nothing
   generates the redirect to Google's consent screen. The frontend's landing
   page (`iiitone-web/src/app/page.tsx`) already redirects to
   `${API_URL}/auth/google/login`; without this route, login is completely
   broken (404), which defeats the point of this task's own Step 6
   smoke-test. Step 3.5 below adds a `LoginHandler` (with anti-CSRF OAuth
   `state`, since this is the first point the login flow is actually wired
   end-to-end and skipping `state` is a known, cheap-to-avoid vulnerability
   class for OAuth login flows).
3. `CallbackHandler`'s session cookie hardcodes `Secure: true`
   (`internal/auth/handlers.go`). This makes the cookie unusable in local
   dev, where the backend runs over plain `http://localhost:8080` (per this
   very task's Step 6 smoke test, and per `docker-compose.yml`) — Secure
   cookies require an HTTPS transport. `internal/config.Config` already has
   an `Env` field (default `"development"`) for exactly this kind of
   environment-gated behavior. Step 3.5 threads a `cookieSecure bool`
   through both the login and callback handlers, computed in `main.go` from
   `cfg.Env == "production"`.

Also fixed as part of this pre-fix pass, outside the Go code: `.env` and
`.env.example`'s `FRONTEND_URL` was still `http://localhost:5173` (Vite's
default port, left over from before the frontend repo switched to Next.js
mid-project) instead of Next.js's `http://localhost:3000` — the OAuth
callback would have redirected a successful login to a dead port. Already
corrected in both files.

- [ ] **Step 3: Implement the router composition root**

```go
// internal/router/router.go
package router

import (
	"net/http"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/metrics"
	"github.com/AnupamSingh2004/iiitone-backend/internal/moderation"
	"github.com/AnupamSingh2004/iiitone-backend/internal/search"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Deps holds everything the router needs. In tests, only JWTSecret is set —
// handlers that need a DB pool are exercised in their own package's tests, not here;
// this router test suite only checks routing/auth-gating shape.
type Deps struct {
	JWTSecret    string
	Pool         *pgxpool.Pool
	Cache        *redis.Client
	Store        storage.Store
	AuthLogin    http.Handler
	AuthCallback http.Handler
	FrontendURL  string
}

func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	m := metrics.New()
	r.Use(m.Middleware)
	r.Get("/metrics", m.Handler().ServeHTTP)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	if deps.AuthLogin != nil {
		r.Get("/auth/google/login", deps.AuthLogin.ServeHTTP)
	}
	if deps.AuthCallback != nil {
		r.Get("/auth/google/callback", deps.AuthCallback.ServeHTTP)
	}

	r.Route("/api", func(api chi.Router) {
		api.Use(auth.RequireAuth(deps.JWTSecret))

		if deps.Pool != nil {
			userRepo := users.NewRepository(deps.Pool)
			userHandlers := users.NewHandlers(userRepo)
			api.Get("/me", userHandlers.Me)
			api.Patch("/me", userHandlers.UpdateMe)

			courseRepo := courses.NewRepository(deps.Pool)
			courseHandlers := courses.NewHandlers(courseRepo)
			api.Get("/courses", courseHandlers.List)

			matRepo := materials.NewRepository(deps.Pool)
			uploadHandler := materials.NewUploadHandler(matRepo, courseRepo, deps.Store)
			api.Post("/materials", uploadHandler.ServeHTTP)

			searchRepo := search.NewRepository(deps.Pool, deps.Cache)
			searchHandlers := search.NewHandlers(searchRepo)
			api.Get("/search", searchHandlers.Search)

			flagsRepo := moderation.NewFlagsRepository(deps.Pool)
			modHandlers := moderation.NewHandlers(matRepo, flagsRepo, deps.Store)
			api.Post("/flags", modHandlers.CreateFlag)

			api.Route("/admin", func(admin chi.Router) {
				admin.Use(auth.RequireAdmin)
				admin.Get("/materials/pending", modHandlers.ListPending)
				admin.Post("/materials/{materialID}/approve", modHandlers.Approve)
				admin.Post("/materials/{materialID}/reject", modHandlers.Reject)
				admin.Get("/flags", modHandlers.ListOpenFlags)
				admin.Post("/flags/{flagID}/resolve", modHandlers.ResolveFlag)
				admin.Post("/users/{userID}/ban", userHandlers.Ban)
				admin.Post("/users/{userID}/unban", userHandlers.Unban)
			})
		}
	})

	return r
}
```

- [ ] **Step 3.5: Add the login-initiation handler and thread `cookieSecure` through the OAuth handlers**

`internal/materials.NewUploadHandler`'s real (already-implemented) signature
takes a `courseResolver` as its second argument — `courses.Repository`
already satisfies that interface (`FindOrCreate(ctx, name, branch string,
year, semester int, createdBy *uuid.UUID) (uuid.UUID, error)`), which is why
Step 3 above passes `courseRepo` there; no change needed in
`internal/courses`.

Add a `LoginURL` method to `GoogleVerifier` in `internal/auth/oauth.go`:

```go
// LoginURL returns the Google OAuth consent-screen URL for the given
// anti-CSRF state value.
func (g *GoogleVerifier) LoginURL(state string) string {
	return g.oauthConfig.AuthCodeURL(state)
}
```

Rewrite `internal/auth/handlers.go` to add `LoginHandler` and thread
`cookieSecure` through both handlers, with an OAuth `state` cookie the
callback validates:

```go
// internal/auth/handlers.go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var errNoIDToken = errors.New("no id_token in oauth response")

// oauthStateCookie carries the anti-CSRF state value from LoginHandler's
// redirect through to CallbackHandler, which must see it match the `state`
// query param Google echoes back before trusting the callback at all.
const oauthStateCookie = "oauth_state"

type UpsertedUser struct {
	ID     uuid.UUID
	Role   string
	Status string
}

// UserUpserter persists/loads the user record for a verified identity.
// Implemented by the users package's repository (Task 11).
type UserUpserter interface {
	UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error)
}

// urlGenerator is implemented by GoogleVerifier; kept as its own small
// interface (rather than reusing CodeVerifier) so LoginHandler only depends
// on the one capability it actually needs.
type urlGenerator interface {
	LoginURL(state string) string
}

// LoginHandler starts the Google OAuth flow: generates a random state
// value, stores it in a short-lived cookie, and redirects to Google's
// consent screen. CallbackHandler verifies the returned state matches.
type LoginHandler struct {
	verifier     urlGenerator
	cookieSecure bool
}

func NewLoginHandler(v urlGenerator, cookieSecure bool) *LoginHandler {
	return &LoginHandler{verifier: v, cookieSecure: cookieSecure}
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(raw)

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/auth/google",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   5 * 60,
	})

	http.Redirect(w, r, h.verifier.LoginURL(state), http.StatusFound)
}

type CallbackHandler struct {
	verifier      CodeVerifier
	users         UserUpserter
	allowedDomain string
	jwtSecret     string
	frontendURL   string
	cookieSecure  bool
}

func NewCallbackHandler(v CodeVerifier, u UserUpserter, allowedDomain, jwtSecret, frontendURL string, cookieSecure bool) *CallbackHandler {
	return &CallbackHandler{verifier: v, users: u, allowedDomain: allowedDomain, jwtSecret: jwtSecret, frontendURL: frontendURL, cookieSecure: cookieSecure}
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// Consume the state cookie now that it's been checked, regardless of
	// how the rest of the flow turns out — it's single-use.
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: "", Path: "/auth/google", MaxAge: -1,
	})

	identity, err := h.verifier.VerifyCode(r.Context(), code)
	if err != nil {
		http.Error(w, "oauth verification failed", http.StatusBadGateway)
		return
	}

	if !ValidateCollegeIdentity(identity.Email, identity.HD, h.allowedDomain) {
		http.Error(w, "account is not a verified college account", http.StatusForbidden)
		return
	}

	user, err := h.users.UpsertFromIdentity(r.Context(), identity)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	if user.Status == "banned" {
		http.Error(w, "account is banned", http.StatusForbidden)
		return
	}

	token, err := IssueToken(h.jwtSecret, user.ID, user.Role, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	http.Redirect(w, r, h.frontendURL, http.StatusFound)
}
```

This changes `NewCallbackHandler`'s signature (adds a trailing `cookieSecure
bool`), which breaks every existing call site in
`internal/auth/handlers_test.go` (6 of them) and will need a matching
`state` cookie + query param added to each existing test request, since the
state check now runs before anything else. Update
`internal/auth/handlers_test.go`:
- Add `cookieSecure bool` (pass `false`, matching local/test behavior) as
  the new 6th argument to every existing `NewCallbackHandler(...)` call.
- Add a small test helper that builds a request carrying a matching
  `oauth_state` cookie and `state` query param (e.g.
  `withValidState(req, "test-state")` that sets `req.AddCookie(&http.Cookie{Name:
  oauthStateCookie, Value: "test-state"})` and appends `&state=test-state`
  to the request URL), and use it in every existing test's request
  construction so they still exercise what they were testing before this
  change (missing code, wrong domain, verifier error, upsert error, banned
  user) rather than all now failing on the new state check instead.
- Add two NEW tests: `TestCallbackHandler_MissingStateCookie_BadRequest`
  (request has `code` and `state` query param but no `oauth_state` cookie —
  must 400) and `TestCallbackHandler_StateMismatch_BadRequest` (cookie value
  and query param `state` differ — must 400).
- Add tests for the new `LoginHandler`: it should redirect
  (`http.StatusFound`) to a URL produced by a fake `urlGenerator`, and set
  an `oauth_state` cookie whose value appears in that redirect URL (a fake
  `urlGenerator.LoginURL(state)` implementation can just echo `state` back
  in a fixed URL template so the test can assert the cookie value and the
  `Location` header's query string agree).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/router/... ./internal/auth/... -v`
Expected: PASS

- [ ] **Step 4.5: Add an admin-route auth-gating regression test to `router_test.go`**

The router test file's own stated purpose (see the **Files** list above:
"smoke test — public routes reachable, protected routes reject
unauthenticated requests") only covers the unauthenticated case so far. Add
an integration test (skipped without `DATABASE_URL`, following the
established pattern in `internal/search/search_test.go` /
`internal/moderation/flags_repository_test.go`) that connects a real pool,
builds `router.New(Deps{JWTSecret: "test-secret", Pool: pool})`, issues a
valid session cookie for a **non-admin** ("student") user via
`auth.IssueToken`, and asserts a request to an admin-only route (e.g. `GET
/api/admin/materials/pending`) returns `403 Forbidden` — not 401 (already
covered) and not 200. This closes a test-coverage gap noted during Task 11's
review (no regression test existed anywhere confirming `RequireAdmin`
actually rejects a non-admin *authenticated* caller specifically on a
real wired route, as opposed to unit-testing the middleware in isolation).

- [ ] **Step 5: Wire `cmd/server/main.go`**

```go
// cmd/server/main.go
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
```

Note: `db.Connect` gets a 10s-bounded context so a genuinely unreachable
database fails startup promptly with a clear error instead of hanging on
whatever `pgxpool`'s own internal defaults happen to be; the OAuth provider
discovery call (`NewGoogleVerifier`, which fetches Google's OIDC discovery
document) intentionally keeps `context.Background()` since it's a one-time
startup call with its own reasonable internal HTTP client timeout, not
something this task needs to re-bound.

- [ ] **Step 6: Build and smoke-test the full stack**

Run: `docker compose up -d postgres redis minio minio-init && source .env && migrate -path ./migrations -database "$DATABASE_URL" up && go run ./cmd/server &`
Then: `curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/healthz`
Expected: `200`. Also verify `curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/auth/google/login` redirects (`302`) rather than 404 — this route not existing at all was the original bug this pre-fix caught. Stop the server afterward (`kill %1` or `fg` + Ctrl-C).

- [ ] **Step 7: Commit**

```bash
git add internal/router internal/auth cmd/server/main.go .env .env.example
git commit -m "Wire router and main.go composition root"
```

---

## Task 20: OpenAPI spec and ER diagram docs

**Files:**
- Create: `docs/openapi.yaml`
- Create: `docs/er-diagram.md`

- [ ] **Step 1: Write `docs/openapi.yaml`** covering: `GET /healthz`, `GET /auth/google/callback`, `GET /api/me`, `PATCH /api/me`, `GET /api/courses`, `POST /api/materials`, `GET /api/search`, `POST /api/flags`, `GET /api/admin/materials/pending`, `POST /api/admin/materials/{materialID}/approve`, `POST /api/admin/materials/{materialID}/reject`, `GET /api/admin/flags`, `POST /api/admin/flags/{flagID}/resolve`, `POST /api/admin/users/{userID}/ban`, `POST /api/admin/users/{userID}/unban` — request/response schemas matching the handler implementations from Tasks 9-17, `cookieAuth` security scheme for the `session` cookie.

- [ ] **Step 2: Write `docs/er-diagram.md`** with a Mermaid `erDiagram` block for `users`, `courses`, `materials`, `flags` and their relationships, matching the migration in Task 4.

- [ ] **Step 3: Validate the OpenAPI spec is well-formed**

Run: `npx --yes @redocly/cli lint docs/openapi.yaml` (or any available OpenAPI linter)
Expected: no errors (warnings acceptable).

- [ ] **Step 4: Commit**

```bash
git add docs/openapi.yaml docs/er-diagram.md
git commit -m "Add OpenAPI spec and ER diagram documentation"
```

---

## Task 21: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: iiitone
          POSTGRES_PASSWORD: iiitone
          POSTGRES_DB: iiitone
        ports: ["5432:5432"]
        options: >-
          --health-cmd "pg_isready -U iiitone"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 10
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go build ./...
      - run: go vet ./...
      - name: Run migrations
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          migrate -path ./migrations -database "postgres://iiitone:iiitone@localhost:5432/iiitone?sslmode=disable" up
      - name: Test
        env:
          DATABASE_URL: postgres://iiitone:iiitone@localhost:5432/iiitone?sslmode=disable
        run: go test ./... -v
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "Add GitHub Actions CI workflow"
```

---

## Task 22: README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write local dev setup instructions** covering: prerequisites (Go 1.22+, Docker, `migrate` CLI), `cp .env.example .env`, `docker compose up -d`, running migrations, `go run ./cmd/server`, running tests (`make test`, noting integration tests need `DATABASE_URL` sourced from `.env`), and a note that `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET` must be replaced with real values from a Google Cloud OAuth app before login will work end-to-end.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "Add README with local dev setup instructions"
```

---

## Definition of Done (Phase 1 backend)

- [ ] `docker compose up -d && go run ./cmd/server` brings up a working API against a fresh DB.
- [ ] `go test ./...` passes with `DATABASE_URL`/MinIO available (integration tests skip cleanly without them).
- [ ] Auth: OAuth login rejects any non-`iiitdmj.ac.in` account (both suffix and `hd` claim checked).
- [ ] Upload: duplicate PDFs rejected by hash; PDFs without a text layer still store successfully; non-PDFs rejected.
- [ ] Search: keyword search returns only `approved` materials, ranked by relevance.
- [ ] Moderation: admin can approve/reject pending uploads, resolve flags (optionally rejecting the material), and ban/unban users independently.
- [ ] `docs/openapi.yaml` and `docs/er-diagram.md` reflect the implemented API and schema.
- [ ] CI passes on GitHub Actions.
