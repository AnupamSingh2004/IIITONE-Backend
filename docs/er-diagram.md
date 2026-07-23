# ER Diagram

Entity-relationship diagram for the IIITOne schema, derived from
`migrations/000001_init_schema.up.sql`.

```mermaid
erDiagram
    USERS ||--o{ COURSES : "created_by (nullable)"
    USERS ||--o{ MATERIALS : "uploader_id"
    USERS ||--o{ FLAGS : "reported_by"
    COURSES ||--o{ MATERIALS : "course_id"
    MATERIALS ||--o{ FLAGS : "material_id (ON DELETE CASCADE)"

    USERS {
        uuid id PK
        text email UK "NOT NULL"
        text google_sub UK "NOT NULL"
        text name "NOT NULL"
        text branch "nullable"
        int year "nullable"
        text role "NOT NULL, default 'student', CHECK IN (student, admin)"
        text status "NOT NULL, default 'active', CHECK IN (active, banned)"
        timestamptz created_at "NOT NULL, default now()"
    }

    COURSES {
        uuid id PK
        text code "nullable"
        text name "NOT NULL"
        text branch "NOT NULL"
        int year "NOT NULL"
        int semester "NOT NULL"
        uuid created_by FK "nullable, REFERENCES users(id)"
        timestamptz created_at "NOT NULL, default now()"
    }

    MATERIALS {
        uuid id PK
        uuid uploader_id FK "NOT NULL, REFERENCES users(id)"
        uuid course_id FK "NOT NULL, REFERENCES courses(id)"
        text type "NOT NULL, CHECK IN (notes, pyq, assignment)"
        text title "NOT NULL"
        text file_key "NOT NULL"
        text content_hash UK "NOT NULL"
        bigint file_size "NOT NULL"
        boolean has_text_layer "NOT NULL, default false"
        text extracted_text "nullable"
        tsvector search_vector "nullable, maintained by trigger"
        text status "NOT NULL, default 'pending', CHECK IN (pending, approved)"
        timestamptz created_at "NOT NULL, default now()"
    }

    FLAGS {
        uuid id PK
        uuid material_id FK "NOT NULL, REFERENCES materials(id) ON DELETE CASCADE"
        uuid reported_by FK "NOT NULL, REFERENCES users(id)"
        text reason "NOT NULL"
        text status "NOT NULL, default 'open', CHECK IN (open, resolved)"
        timestamptz created_at "NOT NULL, default now()"
    }
```

## Notes

- **`courses` has a unique constraint on `(name, branch, year, semester)`** —
  this is what makes the on-the-fly "find or create" course resolution in the
  material upload flow (`courses.Repository.FindOrCreate`) race-safe via
  `ON CONFLICT (name, branch, year, semester) DO UPDATE`.
- **`materials.status` only has two values: `pending` and `approved`.** There
  is no `rejected` status — rejecting a material (`POST
  /api/admin/materials/{materialID}/reject`, or resolving a flag with
  `material_id`) hard-deletes the row instead of flagging it as rejected.
  This also frees `content_hash` for the same file to be re-uploaded later.
- **`flags.material_id` has `ON DELETE CASCADE`.** When a material is
  hard-deleted (rejected, or deleted while resolving a flag), any other open
  flags referencing it are automatically removed by the database — the
  application code does not need to clean these up itself.
- **`materials.content_hash` is globally unique.** This is the SHA-256 of the
  uploaded file's bytes, used to reject duplicate uploads before they reach
  storage.
- **`materials.search_vector`** is a `tsvector` populated by the
  `materials_search_vector_trigger` (BEFORE INSERT OR UPDATE), combining
  `title` and `extracted_text`, and indexed with a GIN index
  (`materials_search_idx`) to back `GET /api/search`.
- Additional indexes: `materials_course_idx` on `materials(course_id)`, and
  `courses_lookup_idx` on `courses(branch, year, semester)` (backing `GET
  /api/courses`'s filter).
- **`courses.created_by` is nullable** because it's only populated for
  courses created via the on-the-fly resolution path during material upload
  (`courses.Repository.FindOrCreate` is passed the uploader's ID); it's null
  for any course that predates that flow (e.g. seeded directly).
