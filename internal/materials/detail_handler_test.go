package materials

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeDetailRepo struct {
	detail MaterialDetail
	err    error
	called bool
}

func (f *fakeDetailRepo) GetByID(ctx context.Context, id uuid.UUID) (MaterialDetail, error) {
	f.called = true
	if f.err != nil {
		return MaterialDetail{}, f.err
	}
	return f.detail, nil
}

type fakeURLSigner struct {
	url    string
	err    error
	called bool
}

func (f *fakeURLSigner) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	f.called = true
	if f.err != nil {
		return "", f.err
	}
	return f.url, nil
}

// requestWithMaterialID builds a request routed through chi so
// chi.URLParam(r, "materialID") resolves the way it does in production.
func requestWithMaterialID(idStr string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/materials/"+idStr, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("materialID", idStr)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestDetailHandler_Success_ReturnsFileURL(t *testing.T) {
	id := uuid.New()
	repo := &fakeDetailRepo{detail: MaterialDetail{
		ID: id, Title: "Test Notes", Type: "notes", CourseName: "Data Structures", FileKey: "materials/abc.pdf",
	}}
	signer := &fakeURLSigner{url: "https://storage.example.com/signed-url"}
	h := NewDetailHandler(repo, signer)

	req := requestWithMaterialID(id.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, signer.called)

	var got detailResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, id, got.ID)
	require.Equal(t, "Test Notes", got.Title)
	require.Equal(t, "notes", got.Type)
	require.Equal(t, "Data Structures", got.CourseName)
	require.Equal(t, "https://storage.example.com/signed-url", got.FileURL)
}

func TestDetailHandler_InvalidID_BadRequest(t *testing.T) {
	repo := &fakeDetailRepo{}
	signer := &fakeURLSigner{}
	h := NewDetailHandler(repo, signer)

	req := requestWithMaterialID("not-a-uuid")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.False(t, repo.called)
	require.False(t, signer.called)
}

func TestDetailHandler_NotFound_ReturnsNotFound(t *testing.T) {
	repo := &fakeDetailRepo{err: errors.New("no rows")}
	signer := &fakeURLSigner{}
	h := NewDetailHandler(repo, signer)

	req := requestWithMaterialID(uuid.New().String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.False(t, signer.called)
}

func TestDetailHandler_SigningFails_InternalServerError(t *testing.T) {
	repo := &fakeDetailRepo{detail: MaterialDetail{
		ID: uuid.New(), Title: "Test Notes", Type: "notes", CourseName: "Data Structures", FileKey: "materials/abc.pdf",
	}}
	signer := &fakeURLSigner{err: errors.New("signing failed")}
	h := NewDetailHandler(repo, signer)

	req := requestWithMaterialID(repo.detail.ID.String())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
