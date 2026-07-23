package moderation

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

func TestCreateFlag_AndResolve(t *testing.T) {
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
	flagRepo := NewFlagsRepository(pool)

	uploader, err := userRepo.UpsertFromIdentity(ctx, testIdentity("flag-test-1"))
	require.NoError(t, err)
	courseID, err := courseRepo.FindOrCreate(ctx, "Flag Test Course", "CSE", 2026, 3, nil)
	require.NoError(t, err)
	matID, err := matRepo.Create(ctx, materials.CreateInput{
		UploaderID: uploader.ID, CourseID: courseID, Type: "notes", Title: "Flagged item",
		FileKey: "k-flag", ContentHash: "flag-hash-1", FileSize: 10,
	})
	require.NoError(t, err)
	// Deleting the material cascades (ON DELETE CASCADE) to any flags on it, so
	// this single cleanup keeps re-runs against a persistent local DB idempotent.
	t.Cleanup(func() { require.NoError(t, matRepo.Delete(context.Background(), matID)) })
	require.NoError(t, matRepo.Approve(ctx, matID))

	flagID, err := flagRepo.Create(ctx, matID, uploader.ID, "wrong course tag")
	require.NoError(t, err)

	open, err := flagRepo.ListOpen(ctx)
	require.NoError(t, err)
	found := false
	for _, f := range open {
		if f.ID == flagID {
			require.Equal(t, matID, f.MaterialID)
			require.Equal(t, "Flagged item", f.MaterialTitle)
			require.Equal(t, uploader.ID, f.UploaderID, "UploaderID must be the flagged material's uploader, not the reporter")
			require.Equal(t, "wrong course tag", f.Reason)
			found = true
		}
	}
	require.True(t, found)

	require.NoError(t, flagRepo.Resolve(ctx, flagID))
	open, err = flagRepo.ListOpen(ctx)
	require.NoError(t, err)
	for _, f := range open {
		require.NotEqual(t, flagID, f.ID)
	}
}
