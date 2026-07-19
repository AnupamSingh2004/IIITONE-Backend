package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_ValidCookie_CallsNext(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	token, _ := IssueToken(secret, userID, "student", time.Hour)

	var gotUserID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := ClaimsFromContext(r.Context())
		gotUserID = claims.UserID
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/app/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	RequireAuth(secret)(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, userID, gotUserID)
}

func TestRequireAuth_MissingCookie_Rejected(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/app/me", nil)
	rec := httptest.NewRecorder()

	RequireAuth("test-secret")(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAdmin_NonAdmin_Rejected(t *testing.T) {
	secret := "test-secret"
	token, _ := IssueToken(secret, uuid.New(), "student", time.Hour)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	RequireAuth(secret)(RequireAdmin(next)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}
