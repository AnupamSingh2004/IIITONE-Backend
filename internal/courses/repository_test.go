package courses

import (
	"context"
	"os"
	"sync"
	"testing"

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

func TestFindOrCreate_ConcurrentSameCourse_ResolvesToOneRow(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id, err := repo.FindOrCreate(ctx, "Race Condition Course", "CSE", 2026, 3, nil)
			require.NoError(t, err)
			ids[idx] = id.String()
		}(i)
	}
	wg.Wait()

	first := ids[0]
	for _, id := range ids {
		require.Equal(t, first, id, "all concurrent find-or-create calls must resolve to the same course id")
	}
}

func TestList_FiltersByBranchYearSemester(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	_, err := repo.FindOrCreate(ctx, "Data Structures", "CSE", 2026, 3, nil)
	require.NoError(t, err)

	list, err := repo.List(ctx, "CSE", 2026, 3)
	require.NoError(t, err)
	require.NotEmpty(t, list)
	for _, c := range list {
		require.Equal(t, "CSE", c.Branch)
	}
}
