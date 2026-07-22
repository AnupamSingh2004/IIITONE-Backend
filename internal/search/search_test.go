package search

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
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

	// course_id/type filters must actually narrow results, not be silently ignored.
	wrongType := "pyq"
	filtered, err := searchRepo.Query(ctx, Query{Text: "deadlock", Type: &wrongType})
	require.NoError(t, err)
	require.Empty(t, filtered, "type filter must exclude a non-matching type")
}
