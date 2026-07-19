# IIITOne — Backend

Go REST API for **IIITOne**, the academic resource hub for IIIT Jabalpur (IIITDMJ) students — Google-Workspace-gated login, PDF upload with dedup and text-layer extraction, keyword search, and a manual admin moderation queue.

This repo covers **Phase 1 (Academic Resource Hub)** only. Marketplace and hardening are later phases — see `docs/superpowers/specs/` for the full design spec.

> **Status: work in progress.** Auth (OAuth callback, JWT, hd-claim validation), the DB schema, and object storage are implemented and tested. Materials/courses/users repositories, the upload/search/moderation HTTP handlers, and the `main.go` router wiring are still in progress — `go run ./cmd/server` does not yet serve a functioning API end-to-end. Check `docs/superpowers/plans/2026-07-17-backend-phase1.md` for the up-to-date task list and what's done.

## Tech stack

- Go 1.26, [chi](https://github.com/go-chi/chi) router
- PostgreSQL via [pgx](https://github.com/jackc/pgx) + [golang-migrate](https://github.com/golang-migrate/migrate)
- Redis ([go-redis](https://github.com/redis/go-redis)) for search result caching
- MinIO ([minio-go](https://github.com/minio/minio-go)) for object storage locally, S3-compatible — swappable for Azure Blob in production behind the `internal/storage.Store` interface
- Google OAuth via [go-oidc](https://github.com/coreos/go-oidc) + `golang.org/x/oauth2`, sessions via [golang-jwt](https://github.com/golang-jwt/jwt) (HS256, pinned)
- PDF text-layer extraction via [pdfcpu](https://github.com/pdfcpu/pdfcpu)
- Docker Compose for local dev infrastructure

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) — `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`, then make sure `$(go env GOPATH)/bin` is on your `PATH`

## Local dev setup

```bash
# 1. Copy env config
cp .env.example .env

# 2. Bring up Postgres, Redis, and MinIO
docker compose up -d postgres redis minio minio-init

# 3. Run migrations
set -a && source .env && set +a
migrate -path ./migrations -database "$DATABASE_URL" up

# 4. Run the server (currently a placeholder — see Status above)
go run ./cmd/server

# 5. Run tests (integration tests need the containers above running and .env sourced)
set -a && source .env && set +a
go test ./... -v
```

**Note on ports:** `docker-compose.yml` maps Postgres to host port **5434** (not the default 5432) — this machine already runs Postgres containers for other local projects on 5432/5433. If you're on a clean machine, feel free to remap to 5432 in both `docker-compose.yml` and `.env`/`.env.example`.

**Google OAuth:** `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET` in `.env.example` are placeholders. To exercise the real OAuth flow (not just the unit tests, which fake the verifier), create a Google Cloud OAuth 2.0 Client ID restricted to `@iiitdmj.ac.in`-style usage and drop the real credentials into your local `.env`.

## Project layout

```
cmd/server/         entrypoint (main.go)
internal/
  auth/              Google OAuth callback, hd-claim + domain validation, JWT issue/verify, session middleware
  config/            env-based config loader
  db/                Postgres connection pool
  storage/           object storage interface + MinIO implementation
  (users, courses, materials, search, moderation, metrics, router — in progress)
migrations/          golang-migrate SQL files
docs/
  superpowers/specs/  approved design spec
  superpowers/plans/  implementation plan (task-by-task, with checkboxes)
```

## Testing

Tests are split into pure unit tests (run always) and integration tests (skip cleanly if `DATABASE_URL`/`STORAGE_ENDPOINT` aren't set):

```bash
go test ./...              # unit tests only, integration tests skip
set -a && source .env && set +a
go test ./... -v           # full suite against local docker-compose infra
```

## Deployment target (not yet wired)

Per the design spec: backend → Azure Container Apps + Azure Database for PostgreSQL + Azure Cache for Redis + Azure Blob Storage (behind the same `storage.Store` interface used for MinIO locally). Frontend (`iiitone-web`) → Cloudflare Pages. Deployment pipelines aren't set up yet — this is a target, not a working CI/CD flow.
