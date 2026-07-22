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

// Result matches the frontend's MaterialCard shape exactly (see
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
			results := []Result{}
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

	results := []Result{}
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
