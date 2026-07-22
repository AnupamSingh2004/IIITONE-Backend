package search

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/redis/go-redis/v9"
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
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), approvedID)) })
	require.NoError(t, matRepo.Approve(ctx, approvedID))

	pendingID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title: "Operating Systems Scheduling Notes (still pending)", FileKey: "k2", ContentHash: "search-hash-2", FileSize: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), pendingID)) })

	results, err := searchRepo.Query(ctx, Query{Text: "deadlock"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, approvedID, results[0].ID)
	// Result must carry enough to render a card without a second round-trip —
	// the frontend's MaterialCard needs type/courseName/hasTextLayer directly.
	require.Equal(t, "notes", results[0].Type)
	require.Equal(t, "Operating Systems", results[0].CourseName)
	require.False(t, results[0].HasTextLayer)
	require.Greater(t, results[0].Rank, float64(0), "a matching result must carry a positive ts_rank")

	// course_id/type filters must actually narrow results, not be silently ignored.
	wrongType := "pyq"
	filtered, err := searchRepo.Query(ctx, Query{Text: "deadlock", Type: &wrongType})
	require.NoError(t, err)
	require.Empty(t, filtered, "type filter must exclude a non-matching type")
}

func TestSearch_OrdersByRankDescending(t *testing.T) {
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
	searchRepo := NewRepository(pool, nil)

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity())
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Rank Test Course", "CSE", 2026, 5, nil)
	require.NoError(t, err)

	// A title where the search term is repeated (and is the dominant content)
	// should out-rank a title where the term appears once alongside a lot of
	// unrelated text, proving ORDER BY rank DESC is actually doing something
	// rather than returning matches in insertion/scan order.
	strongID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title: "Thermodynamics Thermodynamics Thermodynamics", FileKey: "k-rank-strong", ContentHash: "search-hash-rank-strong", FileSize: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), strongID)) })
	require.NoError(t, matRepo.Approve(ctx, strongID))

	weakID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title:   "Assignment covering various mechanics and a passing mention of thermodynamics",
		FileKey: "k-rank-weak", ContentHash: "search-hash-rank-weak", FileSize: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), weakID)) })
	require.NoError(t, matRepo.Approve(ctx, weakID))

	results, err := searchRepo.Query(ctx, Query{Text: "thermodynamics"})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, strongID, results[0].ID, "the title dominated by the search term must rank first")
	require.Equal(t, weakID, results[1].ID)
	require.Greater(t, results[0].Rank, results[1].Rank)
}

func TestSearch_Cache_HitAvoidsDBAndCorruptValueFallsThrough(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dbURL == "" || redisAddr == "" {
		t.Skip("DATABASE_URL/REDIS_ADDR not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	cache := redis.NewClient(&redis.Options{Addr: redisAddr})
	t.Cleanup(func() { require.NoError(t, cache.Close()) })

	userRepo := users.NewRepository(pool)
	courseRepo := courses.NewRepository(pool)
	matRepo := materials.NewRepository(pool)
	searchRepo := NewRepository(pool, cache)

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity())
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Cache Test Course", "CSE", 2026, 5, nil)
	require.NoError(t, err)

	matID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes",
		Title: "Caching Behavior Notes", FileKey: "k-cache-1", ContentHash: "search-hash-cache-1", FileSize: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), matID)) })
	require.NoError(t, matRepo.Approve(ctx, matID))

	query := Query{Text: "caching"}
	cacheKey := cacheKeyFor(query)
	t.Cleanup(func() { cache.Del(context.Background(), cacheKey) })

	first, err := searchRepo.Query(ctx, query)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, matID, first[0].ID)

	// The write-through on miss must have populated the cache under the
	// exact key cacheKeyFor derives for this query.
	cached, err := cache.Get(ctx, cacheKey).Result()
	require.NoError(t, err, "a successful query must populate the cache")
	require.Contains(t, cached, matID.String())

	// Hard-delete the material from Postgres directly, bypassing the repo,
	// so a second Query() can only produce this result via the cache.
	_, err = pool.Exec(ctx, "DELETE FROM materials WHERE id = $1", matID)
	require.NoError(t, err)

	second, err := searchRepo.Query(ctx, query)
	require.NoError(t, err)
	require.Equal(t, first, second, "a cache hit must be served without touching Postgres")

	// A corrupted cache value must not be returned as-is; Query must fall
	// through to Postgres (which now has nothing left, since it was deleted
	// above) rather than propagating garbage or erroring.
	require.NoError(t, cache.Set(ctx, cacheKey, "not valid json", 0).Err())
	third, err := searchRepo.Query(ctx, query)
	require.NoError(t, err)
	require.Empty(t, third, "a corrupt cache entry must fall through to the DB, not be returned verbatim")
}
