package moderation

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// withURLParam injects a chi URL param the way the real router would, so
// handlers reading chi.URLParam(r, ...) can be exercised directly with
// httptest instead of a live router.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// authedRequest builds a request carrying a valid session cookie and routes
// it through the real RequireAuth middleware (see internal/users/handlers_test.go
// for the pattern this follows).
func authedRequest(t *testing.T, secret, method, target string, body []byte) *http.Request {
	t.Helper()
	token, err := auth.IssueToken(secret, uuid.New(), "student", time.Hour)
	require.NoError(t, err)
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	return req
}

func TestApprove_InvalidMaterialID_BadRequest(t *testing.T) {
	// repo is never reached once materialID fails to parse, so a nil
	// *materials.Repository is safe here.
	h := NewHandlers(nil, nil, nil)
	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/admin/materials/not-a-uuid/approve", nil), "materialID", "not-a-uuid")
	rec := httptest.NewRecorder()

	h.Approve(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestReject_InvalidMaterialID_BadRequest(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/admin/materials/not-a-uuid/reject", nil), "materialID", "not-a-uuid")
	rec := httptest.NewRecorder()

	h.Reject(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResolveFlag_InvalidFlagID_BadRequest(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/admin/flags/not-a-uuid/resolve", nil), "flagID", "not-a-uuid")
	rec := httptest.NewRecorder()

	h.ResolveFlag(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResolveFlag_InvalidMaterialIDQueryParam_BadRequest(t *testing.T) {
	h := NewHandlers(nil, nil, nil)
	flagID := uuid.New().String()
	req := withURLParam(
		httptest.NewRequest(http.MethodPost, "/api/admin/flags/"+flagID+"/resolve?material_id=not-a-uuid", nil),
		"flagID", flagID,
	)
	rec := httptest.NewRecorder()

	h.ResolveFlag(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "an invalid material_id must 400 before ever touching the flags repo")
}

func TestCreateFlag_Unauthenticated_Unauthorized(t *testing.T) {
	// No auth middleware, no session cookie: the handler itself must reject
	// unauthenticated requests before ever dereferencing claims or touching
	// the flags repo (a nil claims pointer would otherwise panic here).
	h := NewHandlers(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/flags", bytes.NewReader([]byte(`{"material_id":"`+uuid.New().String()+`","reason":"x"}`)))
	rec := httptest.NewRecorder()

	h.CreateFlag(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCreateFlag_MalformedBody_BadRequest(t *testing.T) {
	secret := "test-secret"
	h := NewHandlers(nil, nil, nil)
	req := authedRequest(t, secret, http.MethodPost, "/api/admin/flags", []byte(`not json`))
	rec := httptest.NewRecorder()

	auth.RequireAuth(secret)(http.HandlerFunc(h.CreateFlag)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateFlag_InvalidMaterialID_BadRequest(t *testing.T) {
	secret := "test-secret"
	h := NewHandlers(nil, nil, nil)
	req := authedRequest(t, secret, http.MethodPost, "/api/admin/flags", []byte(`{"material_id":"not-a-uuid","reason":"x"}`))
	rec := httptest.NewRecorder()

	auth.RequireAuth(secret)(http.HandlerFunc(h.CreateFlag)).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
