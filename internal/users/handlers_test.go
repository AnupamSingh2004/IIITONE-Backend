package users

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// authedRequest builds a request carrying a valid session cookie and routes
// it through the real RequireAuth middleware, so these tests exercise the
// handler exactly as it will run in production, not with a hand-faked context.
func authedRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()
	token, err := auth.IssueToken(secret, uuid.New(), "student", time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPatch, "/api/me", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	return req
}

func TestUpdateMe_MissingBranch_BadRequest(t *testing.T) {
	secret := "test-secret"
	h := NewHandlers(nil)
	rec := httptest.NewRecorder()

	req := authedRequest(t, secret, []byte(`{"branch":"","year":3}`))
	auth.RequireAuth(secret)(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpdateMe_YearOutOfRange_BadRequest(t *testing.T) {
	secret := "test-secret"
	h := NewHandlers(nil)

	for _, year := range []int{0, -1, 7, 99} {
		body, err := json.Marshal(map[string]any{"branch": "CSE", "year": year})
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		req := authedRequest(t, secret, body)
		auth.RequireAuth(secret)(http.HandlerFunc(h.UpdateMe)).ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "year=%d must be rejected", year)
	}
}
