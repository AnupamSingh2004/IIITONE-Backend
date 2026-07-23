# IIITOne — Backend

Go REST API for **IIITOne**, the academic resource hub for IIIT Jabalpur (IIITDMJ) students — Google-Workspace-gated login, PDF upload with dedup and text-layer extraction, keyword search, and an admin moderation queue (approve/reject uploads, resolve content flags, ban/unban users).

This repo covers **Phase 1 (Academic Resource Hub)** only. Marketplace and hardening are later phases — see `docs/superpowers/specs/` for the full design spec.

> **Status: Phase 1 backend complete.** `docker compose up -d && go run ./cmd/server` brings up a fully working API — auth, uploads, search, moderation, and metrics are all implemented, tested, and wired into the router. See `docs/openapi.yaml` for the full API surface and `docs/superpowers/plans/2026-07-17-backend-phase1.md` for the task-by-task build history.

## Tech stack

- Go 1.26, [chi](https://github.com/go-chi/chi) router
- PostgreSQL via [pgx](https://github.com/jackc/pgx) + [golang-migrate](https://github.com/golang-migrate/migrate), full-text search via Postgres `tsvector`/GIN index
- Redis ([go-redis](https://github.com/redis/go-redis)) for search result caching
- MinIO ([minio-go](https://github.com/minio/minio-go)) for object storage locally, S3-compatible — swappable for Azure Blob in production behind the `internal/storage.Store` interface (including presigned download URLs)
- Google OAuth via [go-oidc](https://github.com/coreos/go-oidc) + `golang.org/x/oauth2`, with anti-CSRF `state` handling; sessions via [golang-jwt](https://github.com/golang-jwt/jwt) (HS256, pinned) in an httpOnly cookie
- PDF text-layer extraction via [pdfcpu](https://github.com/pdfcpu/pdfcpu)
- [Prometheus client_golang](https://github.com/prometheus/client_golang) for `/metrics`
- Docker Compose for local dev infrastructure; GitHub Actions CI (`.github/workflows/ci.yml`)

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

# 4. Run the server
go run ./cmd/server
# -> listening on :8080; try curl http://localhost:8080/healthz

# 5. Run tests (integration tests need the containers above running and .env sourced)
set -a && source .env && set +a
go test ./... -v
```

**Note on ports:** `docker-compose.yml` maps Postgres to host port **5434** (not the default 5432) — this machine already runs Postgres containers for other local projects on 5432/5433. If you're on a clean machine, feel free to remap to 5432 in both `docker-compose.yml` and `.env`/`.env.example`.

**Google OAuth:** `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET` in `.env.example` are placeholders (`replace-me`). The server starts and `/healthz` works fine with placeholders, but login will not — you must replace them with real values from a Google Cloud OAuth 2.0 Client ID before `GET /auth/google/login` → `GET /auth/google/callback` will work end-to-end. `GOOGLE_REDIRECT_URL` must match the client's configured redirect URI exactly (`http://localhost:8080/auth/google/callback` for local dev). `GOOGLE_ALLOWED_DOMAIN` gates login to that Google Workspace domain, checked against both the account's email suffix and its `hd` claim.

**Cookies over local HTTP:** the session cookie's `Secure` flag is computed from `ENV` (`Secure: true` only when `ENV=production`), so login works over plain `http://localhost` in dev without any extra setup.

## Project layout

```
cmd/server/           entrypoint (main.go) — composition root, wires config/db/storage/oauth into the router
internal/
  auth/                Google OAuth login+callback (with anti-CSRF state), hd-claim + domain validation, JWT issue/verify, session middleware
  config/              env-based config loader
  db/                  Postgres connection pool
  storage/             object storage interface (Put/Get/Delete/Exists/PresignedGetURL) + MinIO implementation
  users/               profile get/update, admin ban/unban
  courses/             course catalog lookup + race-safe find-or-create
  materials/           PDF upload (dedup by content hash, text extraction), material detail (presigned file URL)
  search/              Postgres full-text search with Redis result caching
  moderation/          pending-queue approve/reject, content flags, resolve
  metrics/             Prometheus /metrics + HTTP request counter middleware
  router/               chi router composition — every route lives here
migrations/            golang-migrate SQL files
docs/
  openapi.yaml          full API spec (routes, request/response schemas, auth)
  er-diagram.md          schema ER diagram (Mermaid) with design-decision notes
  superpowers/specs/     approved design spec
  superpowers/plans/     implementation plan (task-by-task, with checkboxes)
```

## Testing

Tests are split into pure unit tests (run always) and integration tests (skip cleanly if `DATABASE_URL`/`REDIS_ADDR`/`STORAGE_ENDPOINT` aren't set):

```bash
go test ./...              # unit tests only, integration tests skip
set -a && source .env && set +a
go test ./... -v           # full suite against local docker-compose infra
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go build`, `go vet`, and the full test suite against a fresh Postgres service container on every push/PR to `main`.

## API documentation

- **`docs/openapi.yaml`** — every route, request/response schema, and auth requirement, derived directly from the handler code (not just this README's summary). Lint it with `npx @redocly/cli lint docs/openapi.yaml`.
- **`docs/er-diagram.md`** — the schema as a Mermaid ER diagram, with notes on non-obvious design decisions (e.g. why rejecting a material is a hard delete rather than a status flag, why `flags.material_id` cascades).

## Deployment target (not yet wired)

Per the design spec: backend → Azure Container Apps + Azure Database for PostgreSQL + Azure Cache for Redis + Azure Blob Storage (behind the same `storage.Store` interface used for MinIO locally). Frontend (`iiitone-web`) → Cloudflare Pages. CI runs tests on every push, but there's no deployment pipeline yet — that's a follow-up, not part of this Phase 1 plan.
