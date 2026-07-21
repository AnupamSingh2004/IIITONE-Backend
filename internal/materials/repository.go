package materials

import (
	"context"
	"strconv"

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

// Material's field names/JSON shape must match iiitone-web's MaterialSummary
// (id, title, type, courseName, hasTextLayer) — see internal/search's Result
// struct (Task 16) for the established lowercase-JSON convention this repo
// follows; this type is consumed by the moderation/materials handlers (Tasks
// 15, 17), which are responsible for the actual JSON serialization, so this
// struct itself doesn't need json tags, but keep field naming consistent for
// whoever writes those handlers.
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
		query += ` AND course_id = $` + strconv.Itoa(argN)
		args = append(args, *f.CourseID)
		argN++
	}
	if f.Type != nil {
		query += ` AND type = $` + strconv.Itoa(argN)
		args = append(args, *f.Type)
		argN++
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Material{}
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

	list := []Material{}
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
// flag — flags.material_id has ON DELETE CASCADE, so any open flags on this
// material are cleaned up automatically by the database.
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM materials WHERE id = $1`, id)
	return err
}

// MaterialDetail is the JSON-serializable shape returned by GET
// /api/materials/:id. Field names must match iiitone-web's
// src/app/app/materials/[id]/page.tsx expectations (id, title, type,
// courseName, fileUrl) — note fileUrl is derived from FileKey by the handler
// (e.g. a signed storage URL), not stored directly on this struct.
type MaterialDetail struct {
	ID         uuid.UUID
	Title      string
	Type       string
	CourseName string
	FileKey    string
	Status     string
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (MaterialDetail, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.title, m.type, c.name, m.file_key, m.status
		FROM materials m
		JOIN courses c ON c.id = m.course_id
		WHERE m.id = $1
	`, id)
	var d MaterialDetail
	if err := row.Scan(&d.ID, &d.Title, &d.Type, &d.CourseName, &d.FileKey, &d.Status); err != nil {
		return MaterialDetail{}, err
	}
	return d, nil
}
