# IIITOne — Phase 1 Design: Academic Resource Hub

Status: Approved for spec review
Date: 2026-07-17
Scope: Phase 1 only (Academic Resource Hub). Marketplace (Phase 2) and hardening (Phase 3) are explicitly out of scope for this spec — see "Non-goals" below.

## Context

IIITOne is a two-vertical campus platform for IIIT Jabalpur (IIITDMJ) students: an Academic Resource Hub (PYQs/notes/material search) and a Campus Marketplace, sharing one auth/notification backbone. The full product is too large for a single spec, so per the project's own phased plan, this document covers **Phase 1 — Academic Resource Hub — only**, to be shipped fully before Phase 2 (Marketplace) begins.

Repos are created locally (git-initialized) but **not pushed to GitHub yet** — the user wants to review the scaffold before anything goes remote. Git identity for all repos: `AnupamSingh2004` / `sanupam2004@gmail.com` (already the machine's global git config).

## Non-goals (Phase 1)

- No marketplace (listings, chat, ratings) — Phase 2.
- No mobile app — deferred entirely, not even scaffolded, until after web is solid.
- No semantic/embeddings search — keyword search (Postgres full-text) only. Schema is shaped so a `pgvector` column can be added later without a rewrite.
- No in-app payments (permanent non-goal for the whole product, not just Phase 1).
- No automated ML moderation — manual admin review queue only.
- No production deploy automation in this spec beyond describing the target (Azure + Cloudflare); actual CI/CD secrets and pipelines are an implementation-time task.
- No WebSocket/realtime protocol in Phase 1 (no chat, nothing realtime exists in this vertical) — full WebSocket protocol spec is a Phase 2 (Marketplace chat) artifact, not produced here.

## Repos

- `iiitone-backend` — Go. REST API, auth, upload/search/moderation logic, DB migrations, local dev docker-compose, OpenAPI spec, ER diagram.
- `iiitone-web` — React + TypeScript. Student-facing app + role-gated admin routes.
- No `iiitone-mobile` repo in Phase 1.

## Data Model (Phase 1 scope)

```sql
users
  id UUID PK
  email TEXT UNIQUE NOT NULL        -- must end @iiitdmj.ac.in
  google_sub TEXT UNIQUE NOT NULL
  name TEXT NOT NULL
  branch TEXT
  year INT
  role TEXT NOT NULL DEFAULT 'student'   -- 'student' | 'admin'
  status TEXT NOT NULL DEFAULT 'active'  -- 'active' | 'banned'
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()

courses
  id UUID PK
  code TEXT                              -- nullable; seeded courses have a real code, user-added ones may not
  name TEXT NOT NULL
  branch TEXT NOT NULL
  year INT NOT NULL
  semester INT NOT NULL
  created_by UUID REFERENCES users(id)   -- who added it (seed = null/admin)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  UNIQUE (name, branch, year, semester)  -- find-or-create key; code plays no role in identity

materials
  id UUID PK
  uploader_id UUID NOT NULL REFERENCES users(id)
  course_id UUID NOT NULL REFERENCES courses(id)
  type TEXT NOT NULL                 -- 'notes' | 'pyq' | 'assignment'
  title TEXT NOT NULL
  file_key TEXT NOT NULL             -- object storage path
  content_hash TEXT NOT NULL UNIQUE  -- sha256 of raw file bytes, dedup key
  file_size BIGINT NOT NULL
  has_text_layer BOOLEAN NOT NULL DEFAULT false
  extracted_text TEXT                -- null if has_text_layer = false
  search_vector TSVECTOR             -- generated from title + extracted_text
  status TEXT NOT NULL DEFAULT 'pending'  -- 'pending' | 'approved' (only two states; see Moderation —
                                            -- rejection hard-deletes the row rather than setting a 'rejected' status)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()

flags
  id UUID PK
  material_id UUID NOT NULL REFERENCES materials(id)
  reported_by UUID NOT NULL REFERENCES users(id)
  reason TEXT NOT NULL
  status TEXT NOT NULL DEFAULT 'open'   -- 'open' | 'resolved'
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

GIN index on `materials.search_vector` for full-text search; indexes on `materials(course_id)`, `materials(content_hash)`, `courses(branch, year, semester)`.

An ER diagram (Mermaid) will be checked into `iiitone-backend/docs/er-diagram.md` as part of implementation.

## Backend Architecture

Single Go service, domain-packaged (not microservices):

```text
/cmd/server
/internal/auth        - Google OAuth callback, hd-claim + email-suffix validation, JWT issuance, httpOnly cookie session middleware
/internal/users       - profile read/update, admin ban/unban
/internal/courses     - list courses, add-course-on-the-fly
/internal/materials   - upload, dedup, text extraction trigger, listing, detail
/internal/search      - Postgres FTS query building, ranking, Redis query cache
/internal/moderation  - flag creation, admin review-queue endpoints (approve/reject/ban)
/internal/storage     - object storage interface; MinIO impl (local dev), Azure Blob impl (prod) behind the same interface
/internal/db          - migrations + queries (pgx, sqlc or hand-written)
```

### Auth flow

1. Frontend redirects to Google OAuth consent.
2. Google redirects back to `/auth/google/callback` with an auth code.
3. Backend exchanges code for tokens, decodes the ID token.
4. Server-side validation, **both required**: email suffix ends `@iiitdmj.ac.in` **and** the `hd` claim in the decoded ID token equals `iiitdmj.ac.in`. Reject (redirect to an error page) if either fails.
5. Reject if `users.status = 'banned'` for an existing matching user.
6. Upsert user (create on first login, collecting name/branch/year via a post-login onboarding step if not yet set).
7. Issue JWT, set as httpOnly, secure, SameSite=Lax cookie.
8. Redirect to the app.

`GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` are read from env vars with a documented placeholder in `.env.example` — the user will create the actual Google Cloud OAuth app and supply real values later; this is not blocking for building the code.

### Upload flow

1. Client uploads a PDF via multipart form (title, course_id **or** a new-course name + branch/year/semester, type).
2. Backend rejects non-PDF uploads outright via content-type/magic-byte check before any further processing.
3. Backend streams the file while computing SHA-256.
4. If a row with that `content_hash` already exists, reject with a "duplicate" error (no new row, no storage write). Since rejection hard-deletes the row (see Moderation), a previously-rejected file's hash is simply gone from the table and free to resubmit — no special-casing needed here.
5. If the submitted course is a new free-typed name rather than an existing `course_id`, resolve it first via find-or-create: `INSERT INTO courses (...) VALUES (...) ON CONFLICT (name, branch, year, semester) DO UPDATE SET name = EXCLUDED.name RETURNING id` (the no-op `DO UPDATE` is what makes `RETURNING id` work on a conflict) — this makes concurrent submissions of the same new course race-safe; both resolve to the same `course_id` instead of one failing.
6. Attempt text-layer extraction using `pdfcpu` (pure Go, no license concerns for this scale).
   - Text found → store it in `extracted_text`, `has_text_layer = true`, `search_vector` generated from title + extracted text.
   - No text layer, or the file is corrupted/encrypted and `pdfcpu` errors out → degrade gracefully: `has_text_layer = false`, `extracted_text = null`. The upload is **not** failed in this case — the file is still stored and still fully accessible/viewable, just not full-text searchable by content (title search still applies).
7. Upload the raw file to object storage under a content-hash-derived key.
8. Insert `materials` row with `status = 'pending'`.

### Search flow (Phase 1)

- Postgres full-text search (`tsvector`/`plainto_tsquery`, ranked with `ts_rank`) over `materials.search_vector`, filterable by `course_id`/branch/year/semester/type, restricted to `status = 'approved'`.
- Redis caches hot query results (TTL-based) to absorb the pre-exam load spike (spec calls out 50–100x bursts).
- No embeddings/semantic search in Phase 1 — deferred by explicit decision. The schema (a nullable `embedding vector` column could be added later) and the `/internal/search` package boundary are structured so this is additive, not a rewrite.

### Moderation

- `/admin/*` routes, gated by middleware requiring `users.role == 'admin'`. Initially only the project owner has `role = 'admin'` (set directly in the DB or via a seed migration — no self-service admin promotion in Phase 1).
- Pending-uploads queue: list `status = 'pending'` materials. Approve → `status = 'approved'`. Reject → the row and its storage object are **hard-deleted** immediately (no `rejected` status value exists) — this is what frees its `content_hash` for resubmission.
- Flags queue: list `status = 'open'` flags on **already-approved** materials; resolving a flag lets the admin (a) reject the flagged material itself (same reject-and-delete path as above, removing it from search immediately) and/or (b) ban the uploader (`users.status = 'banned'`), independently. These are separate actions — a bad upload can be pulled without necessarily banning the uploader, and vice versa.
- **Banned users' other approved uploads are not retroactively removed.** Banning only blocks future login (auth-callback step 5); it does not change the `status` of that user's existing `approved` materials, since those were already vetted by an admin and remain useful to other students. If a specific upload is itself the problem, it must be rejected individually via the flags queue, not implied by a ban.

## Web App (`iiitone-web`)

- Public: landing + "Sign in with Google" button.
- Authenticated (`/app/*`):
  - Browse/search materials — filter by branch/course/year/semester/type, keyword search box.
  - Material detail page: **inline PDF viewer** (`react-pdf`/pdf.js) so files are readable in-browser regardless of `has_text_layer`, plus a download button.
  - Upload form: course dropdown (searchable) with an "add new course" inline option that creates the course row on submit.
  - Profile page: name/branch/year (editable), own upload history.
- Admin (`/admin/*`, role-gated — non-admins redirected):
  - Pending-uploads review queue (approve/reject).
  - Open-flags queue (resolve, optionally ban).

Session: backend-issued httpOnly JWT cookie; the React app never touches the token directly, relies on the cookie being sent automatically and a `/me` endpoint to fetch current-user state.

## Infra

**Local dev:** `docker-compose.yml` in `iiitone-backend` brings up Postgres, Redis, MinIO (S3-compatible, local object storage stand-in), and the backend container — `docker compose up` is the one-command local stack. `iiitone-web` runs separately via its own dev server (`npm run dev`), pointed at the local backend via `.env`.

**Production targets:**

- Backend (`iiitone-backend`) → **Azure Container Apps**, backed by **Azure Database for PostgreSQL** (managed) and **Azure Cache for Redis** (managed).
- Object storage (prod) → **Azure Blob Storage**, accessed through the same `/internal/storage` interface used for MinIO locally (swap implementation, not callers).
- Frontend (`iiitone-web`) → **Cloudflare Pages**.
- Actual deployment pipelines (GitHub Actions → Azure/Cloudflare, secrets, resource provisioning) are implementation-time work, not fully speced here — this doc fixes the *targets*, not the Terraform/pipeline details.

**CI:** GitHub Actions per repo — backend: `go build`/`go vet`/`go test`; web: lint/typecheck/build/test. Deploy steps added once Azure/Cloudflare resources exist.

**Observability:** Backend exposes `/metrics` (Prometheus) — upload counts, search request counts, approval/rejection counts, daily active users. This is the direct instrumentation for the project's stated success metric ("real usage spike during exam week, tracked via analytics"). Structured JSON logging throughout.

## Testing Strategy

- Backend: table-driven Go unit tests for auth domain/hd-claim validation, dedup logic, text extraction, search ranking; integration tests against a dockerized Postgres.
- Web: Vitest + React Testing Library for upload form, search/filter UI, PDF viewer, admin queue actions.

## Deliverables checklist (from original brief)

- [x] Monorepo→multi-repo split: `iiitone-backend`, `iiitone-web` (mobile deferred).
- [ ] ER diagram (Mermaid) — `iiitone-backend/docs/er-diagram.md`.
- [ ] OpenAPI 3.0 spec for REST endpoints — `iiitone-backend/docs/openapi.yaml`.
- [ ] One-command local dev stack (`docker compose up` in `iiitone-backend`).

(No WebSocket protocol spec in this checklist — see Non-goals; it's a Phase 2 artifact.)

These are implementation-time artifacts, produced during the build (next: implementation plan), not part of this design doc itself.
