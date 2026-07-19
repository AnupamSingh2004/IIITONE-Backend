package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeIdentity struct {
	email string
	hd    string
	sub   string
	name  string
}

type fakeVerifier struct {
	identity fakeIdentity
	err      error
}

func (f fakeVerifier) VerifyCode(ctx context.Context, code string) (Identity, error) {
	if f.err != nil {
		return Identity{}, f.err
	}
	return Identity{
		Email: f.identity.email,
		HD:    f.identity.hd,
		Sub:   f.identity.sub,
		Name:  f.identity.name,
	}, nil
}

type fakeUserUpserter struct {
	upserted Identity
	err      error
	status   string // defaults to "active" if empty
}

func (f *fakeUserUpserter) UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error) {
	f.upserted = id
	if f.err != nil {
		return UpsertedUser{}, f.err
	}
	status := f.status
	if status == "" {
		status = "active"
	}
	return UpsertedUser{ID: uuid.New(), Role: "student", Status: status}, nil
}

func TestCallbackHandler_ValidDomain_SetsCookieAndRedirects(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "http://localhost:5173", rec.Header().Get("Location"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "session", cookies[0].Name)
	require.True(t, cookies[0].HttpOnly)
}

func TestCallbackHandler_WrongDomain_Rejected(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@gmail.com", hd: "gmail.com", sub: "sub123", name: "Nope",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestCallbackHandler_MissingCode_BadRequest(t *testing.T) {
	verifier := fakeVerifier{}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestCallbackHandler_VerifierError_BadGateway(t *testing.T) {
	verifier := fakeVerifier{err: errors.New("oauth exchange failed")}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestCallbackHandler_UpsertError_InternalServerError(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{err: errors.New("db unavailable")}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestCallbackHandler_BannedUser_Rejected(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Banned Student",
	}}
	users := &fakeUserUpserter{status: "banned"}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173")

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Result().Cookies(), "a banned user must never receive a session cookie, even with a valid college domain")
}
