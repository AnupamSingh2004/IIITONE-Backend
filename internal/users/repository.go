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

// Compile-time assertion that *Repository satisfies auth.UserUpserter, the
// interface Task 9's OAuth callback handler depends on.
var _ auth.UserUpserter = (*Repository)(nil)

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

// Profile is the JSON-serializable shape returned by GET /api/me. Field tags
// are lowercase to match the frontend's User interface
// (iiitone-web/src/hooks/use-session.ts).
type Profile struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email"`
	Name   string    `json:"name"`
	Branch *string   `json:"branch,omitempty"`
	Year   *int      `json:"year,omitempty"`
	Role   string    `json:"role"`
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
