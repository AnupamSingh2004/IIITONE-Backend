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
