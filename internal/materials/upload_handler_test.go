package materials

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// NewUploadHandlerForTest wires a handler with no storage backend, for unit
// tests that only exercise validation/dedup logic (storage.Store is
// nil-safe in ServeHTTP: the Put call is skipped when store is nil). Course
// resolution may also be nil for tests that never reach that step (e.g. the
// non-PDF and duplicate-hash rejection tests, which both return before
// resolveCourse is ever called).
func NewUploadHandlerForTest(repo materialsRepo, courses courseResolver) *UploadHandler {
	return &UploadHandler{repo: repo, courses: courses, store: nil}
}

type fakeMaterialsRepo struct {
	existingHashes map[string]bool
	created        []CreateInput
}

func (f *fakeMaterialsRepo) ExistsByContentHash(ctx context.Context, hash string) (bool, error) {
	return f.existingHashes[hash], nil
}
func (f *fakeMaterialsRepo) Create(ctx context.Context, in CreateInput) (uuid.UUID, error) {
	f.created = append(f.created, in)
	return uuid.New(), nil
}

// fakeStore matches the real storage.Store interface (internal/storage/storage.go),
// including the contentType parameter Put gained in Task 6's review.
type fakeStore struct {
	putCalls []fakePutCall
}

type fakePutCall struct {
	key         string
	contentType string
	size        int64
}

func (f *fakeStore) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	f.putCalls = append(f.putCalls, fakePutCall{key: key, contentType: contentType, size: size})
	return nil
}
func (f *fakeStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}
func (f *fakeStore) Delete(ctx context.Context, key string) error         { return nil }
func (f *fakeStore) Exists(ctx context.Context, key string) (bool, error) { return false, nil }
func (f *fakeStore) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "", nil
}

// fakeCourseResolver matches courses.Repository.FindOrCreate's real signature.
type fakeCourseResolver struct {
	returnID uuid.UUID
	calls    []fakeFindOrCreateCall
}

type fakeFindOrCreateCall struct {
	name, branch   string
	year, semester int
	createdBy      *uuid.UUID
}

func (f *fakeCourseResolver) FindOrCreate(ctx context.Context, name, branch string, year, semester int, createdBy *uuid.UUID) (uuid.UUID, error) {
	f.calls = append(f.calls, fakeFindOrCreateCall{name: name, branch: branch, year: year, semester: semester, createdBy: createdBy})
	return f.returnID, nil
}

func buildMultipartUpload(t *testing.T, fieldName, fileName string, content []byte, extraFields map[string]string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for k, v := range extraFields {
		require.NoError(t, writer.WriteField(k, v))
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/materials", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// authedUploadRequest builds a multipart upload request carrying a valid
// session cookie, so it can be routed through the real auth.RequireAuth
// middleware exactly as it runs in production (see
// internal/users/handlers_test.go's authedRequest for the established
// pattern this follows).
func authedUploadRequest(t *testing.T, secret string, fileName string, content []byte, extraFields map[string]string) *http.Request {
	t.Helper()
	token, err := auth.IssueToken(secret, uuid.New(), "student", time.Hour)
	require.NoError(t, err)
	req := buildMultipartUpload(t, "file", fileName, content, extraFields)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	return req
}

func TestUploadHandler_RejectsDuplicateContentHash(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	hash := sha256Hex(pdfBytes)
	repo.existingHashes[hash] = true

	h := NewUploadHandlerForTest(repo, nil)

	req := buildMultipartUpload(t, "file", "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Empty(t, repo.created)
}

func TestUploadHandler_RejectsNonPDF(t *testing.T) {
	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	h := NewUploadHandlerForTest(repo, nil)

	req := buildMultipartUpload(t, "file", "notes.pdf", []byte("not a pdf"), map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.created)
}

func TestUploadHandler_CourseID_Succeeds_StoresWithPDFContentType(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	secret := "test-secret"
	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	store := &fakeStore{}
	courses := &fakeCourseResolver{}
	h := NewUploadHandler(repo, courses, store)

	courseID := uuid.New()
	req := authedUploadRequest(t, secret, "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": courseID.String(),
	})
	rec := httptest.NewRecorder()
	auth.RequireAuth(secret)(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, repo.created, 1)
	require.Equal(t, courseID, repo.created[0].CourseID)
	require.Equal(t, "notes", repo.created[0].Type)
	require.Equal(t, "Test Notes", repo.created[0].Title)
	require.Empty(t, courses.calls, "FindOrCreate must not be called when course_id is present")

	require.Len(t, store.putCalls, 1)
	require.Equal(t, "application/pdf", store.putCalls[0].contentType)
	require.Equal(t, int64(len(pdfBytes)), store.putCalls[0].size)
}

func TestUploadHandler_CourseNameFallback_ResolvesViaFindOrCreate(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	secret := "test-secret"
	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	store := &fakeStore{}
	resolvedCourseID := uuid.New()
	courses := &fakeCourseResolver{returnID: resolvedCourseID}
	h := NewUploadHandler(repo, courses, store)

	req := authedUploadRequest(t, secret, "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes",
		"course_name": "Data Structures", "branch": "CSE", "year": "2", "semester": "3",
	})
	rec := httptest.NewRecorder()
	auth.RequireAuth(secret)(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, courses.calls, 1)
	require.Equal(t, "Data Structures", courses.calls[0].name)
	require.Equal(t, "CSE", courses.calls[0].branch)
	require.Equal(t, 2, courses.calls[0].year)
	require.Equal(t, 3, courses.calls[0].semester)
	require.NotNil(t, courses.calls[0].createdBy)

	require.Len(t, repo.created, 1)
	require.Equal(t, resolvedCourseID, repo.created[0].CourseID)
}

func TestUploadHandler_MissingCourseInfo_BadRequest(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	secret := "test-secret"
	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	h := NewUploadHandler(repo, &fakeCourseResolver{}, &fakeStore{})

	req := authedUploadRequest(t, secret, "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes",
	})
	rec := httptest.NewRecorder()
	auth.RequireAuth(secret)(h).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, repo.created)
}

func TestUploadHandler_Unauthenticated_Unauthorized(t *testing.T) {
	pdfBytes, err := os.ReadFile("testdata/with-text.pdf")
	require.NoError(t, err)

	repo := &fakeMaterialsRepo{existingHashes: map[string]bool{}}
	h := NewUploadHandler(repo, &fakeCourseResolver{}, &fakeStore{})

	req := buildMultipartUpload(t, "file", "notes.pdf", pdfBytes, map[string]string{
		"title": "Test Notes", "type": "notes", "course_id": uuid.New().String(),
	})
	rec := httptest.NewRecorder()
	// No auth middleware, no session cookie: the handler itself must reject
	// unauthenticated uploads once it reaches the point of needing claims.
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Empty(t, repo.created)
}
