package users

import (
	"context"
	"os"
	"testing"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/stretchr/testify/require"
)

func testRepo(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.Connect(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRepository(pool)
}

func TestUpsertFromIdentity_CreatesThenReuses(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	identity := auth.Identity{Email: "newuser@iiitdmj.ac.in", HD: "iiitdmj.ac.in", Sub: "sub-unique-1", Name: "New User"}

	first, err := repo.UpsertFromIdentity(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, "student", first.Role)
	require.Equal(t, "active", first.Status)

	second, err := repo.UpsertFromIdentity(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "second login with same google_sub must return same user")
}
