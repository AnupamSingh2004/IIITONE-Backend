package materials

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/stretchr/testify/require"
)

func testDeps(t *testing.T) (*Repository, *users.Repository, *courses.Repository) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRepository(pool), users.NewRepository(pool), courses.NewRepository(pool)
}

func TestCreate_DuplicateContentHash_Rejected(t *testing.T) {
	repo, userRepo, courseRepo := testDeps(t)
	ctx := context.Background()

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("dup-test-1"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Dedup Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	input := CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "First upload",
		FileKey: "materials/hash1", ContentHash: "fixed-hash-for-dedup-test", FileSize: 100,
	}
	_, err = repo.Create(ctx, input)
	require.NoError(t, err)

	_, err = repo.Create(ctx, input)
	require.Error(t, err, "second insert with same content_hash must fail the unique constraint")
}

func TestListApproved_ExcludesPending(t *testing.T) {
	repo, userRepo, courseRepo := testDeps(t)
	ctx := context.Background()

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("dup-test-2"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "List Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	id, err := repo.Create(ctx, CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "Pending item",
		FileKey: "materials/hash2", ContentHash: "hash-for-list-test", FileSize: 100,
	})
	require.NoError(t, err)

	results, err := repo.ListApproved(ctx, ListFilter{CourseID: &courseID})
	require.NoError(t, err)
	for _, m := range results {
		require.NotEqual(t, id, m.ID, "pending material must not appear in approved listing")
	}

	require.NoError(t, repo.Approve(ctx, id))
	results, err = repo.ListApproved(ctx, ListFilter{CourseID: &courseID})
	require.NoError(t, err)
	found := false
	for _, m := range results {
		if m.ID == id {
			found = true
		}
	}
	require.True(t, found, "approved material must appear in approved listing")
}

func TestGetByID_ReturnsJoinedCourseName(t *testing.T) {
	repo, userRepo, courseRepo := testDeps(t)
	ctx := context.Background()

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("detail-test-1"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Detail Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	id, err := repo.Create(ctx, CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "pyq", Title: "Detail item",
		FileKey: "materials/hash-detail", ContentHash: "hash-for-detail-test", FileSize: 200,
	})
	require.NoError(t, err)
	require.NoError(t, repo.Approve(ctx, id))

	detail, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, detail.ID)
	require.Equal(t, "Detail item", detail.Title)
	require.Equal(t, "pyq", detail.Type)
	require.Equal(t, "Detail Test Course", detail.CourseName)
	require.Equal(t, "materials/hash-detail", detail.FileKey)
	require.Equal(t, "approved", detail.Status)
}
