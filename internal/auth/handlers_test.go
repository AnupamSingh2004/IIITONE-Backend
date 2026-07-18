package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
}

func (f *fakeUserUpserter) UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error) {
	f.upserted = id
	return UpsertedUser{ID: mustUUID(), Role: "student", Status: "active"}, nil
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
