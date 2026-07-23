package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/db"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/stretchr/testify/require"
)

func TestRouter_ProtectedRouteRejectsUnauthenticated(t *testing.T) {
	r := New(Deps{JWTSecret: "test-secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouter_HealthCheckIsPublic(t *testing.T) {
	r := New(Deps{JWTSecret: "test-secret"})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

// TestRouter_AdminRouteRejectsNonAdmin closes a test-coverage gap noted
// during Task 11's review: no regression test existed anywhere confirming
// RequireAdmin actually rejects a non-admin *authenticated* caller
// specifically on a real wired route (as opposed to unit-testing the
// middleware in isolation).
func TestRouter_AdminRouteRejectsNonAdmin(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userRepo := users.NewRepository(pool)
	// A fixed (email, sub) pair, matching the pattern used by the other
	// packages' integration tests (e.g. internal/search/helpers_test.go):
	// UpsertFromIdentity's ON CONFLICT targets google_sub, so a fixed sub
	// paired with a fixed email is required for reruns against a
	// persistent local DB to stay idempotent — a random sub with a fixed
	// email would violate the users_email_key unique constraint on the
	// second run.
	student, err := userRepo.UpsertFromIdentity(ctx, auth.Identity{
		Email: "router-admin-gate@iiitdmj.ac.in", Sub: "router-admin-gate-sub", Name: "Router Test Student",
	})
	require.NoError(t, err)
	require.Equal(t, "student", student.Role, "a freshly upserted user must default to the student role for this test to be meaningful")

	r := New(Deps{JWTSecret: "test-secret", Pool: pool})

	token, err := auth.IssueToken("test-secret", student.ID, student.Role, time.Hour)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/materials/pending", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
