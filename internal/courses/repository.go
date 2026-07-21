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

// Course is the JSON-serializable shape returned by GET /api/courses. Field
// tags are lowercase to match the frontend's expectations (id, name are what
// iiitone-web/src/hooks/use-materials.ts's useCourses and
// iiitone-web/src/components/materials/CourseCombobox.tsx's CourseOption
// actually read; the remaining fields are exposed for future consumers,
// following the lowercase JSON convention established in Task 11's Profile
// fix, internal/users/repository.go).
type Course struct {
	ID       uuid.UUID `json:"id"`
	Code     *string   `json:"code,omitempty"`
	Name     string    `json:"name"`
	Branch   string    `json:"branch"`
	Year     int       `json:"year"`
	Semester int       `json:"semester"`
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

	list := []Course{}
	for rows.Next() {
		var c Course
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Branch, &c.Year, &c.Semester); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}
