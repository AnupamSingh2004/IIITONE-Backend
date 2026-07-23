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

// withValidState attaches a matching oauth_state cookie and `state` query
// param to req, the way a genuine browser round-trip through LoginHandler
// followed by Google's redirect back to CallbackHandler would. Existing
// tests use this so they keep exercising what they exercised before the
// state check was added (missing code, wrong domain, verifier error, upsert
// error, banned user), rather than all failing on the new state check
// instead.
func withValidState(req *http.Request, state string) *http.Request {
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: state})
	q := req.URL.Query()
	q.Set("state", state)
	req.URL.RawQuery = q.Encode()
	return req
}

// sessionCookie finds the "session" cookie among the response's Set-Cookie
// headers, if any. Since CallbackHandler now always clears the oauth_state
// cookie once the state check passes (a Set-Cookie in its own right), tests
// that want to assert "no session was established" must look for the
// specific session cookie rather than asserting no cookies at all.
func sessionCookie(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

func TestCallbackHandler_ValidDomain_SetsCookieAndRedirects(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "http://localhost:5173", rec.Header().Get("Location"))
	cookie := sessionCookie(rec.Result().Cookies())
	require.NotNil(t, cookie, "a valid callback must set a session cookie")
	require.True(t, cookie.HttpOnly)
}

func TestCallbackHandler_WrongDomain_Rejected(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@gmail.com", hd: "gmail.com", sub: "sub123", name: "Nope",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()))
}

func TestCallbackHandler_MissingCode_BadRequest(t *testing.T) {
	verifier := fakeVerifier{}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestCallbackHandler_VerifierError_BadGateway(t *testing.T) {
	verifier := fakeVerifier{err: errors.New("oauth exchange failed")}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()))
}

func TestCallbackHandler_UpsertError_InternalServerError(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{err: errors.New("db unavailable")}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()))
}

func TestCallbackHandler_BannedUser_Rejected(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Banned Student",
	}}
	users := &fakeUserUpserter{status: "banned"}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := withValidState(httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil), "test-state")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()), "a banned user must never receive a session cookie, even with a valid college domain")
}

func TestCallbackHandler_MissingStateCookie_BadRequest(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state=test-state", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()))
}

func TestCallbackHandler_StateMismatch_BadRequest(t *testing.T) {
	verifier := fakeVerifier{identity: fakeIdentity{
		email: "student@iiitdmj.ac.in", hd: "iiitdmj.ac.in", sub: "sub123", name: "Student One",
	}}
	users := &fakeUserUpserter{}
	h := NewCallbackHandler(verifier, users, "iiitdmj.ac.in", "test-secret", "http://localhost:5173", false)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state=query-state", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "cookie-state"})
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, sessionCookie(rec.Result().Cookies()))
}

// fakeURLGenerator is a fake urlGenerator for LoginHandler tests: it echoes
// the given state back into a fixed URL template so a test can assert the
// oauth_state cookie value and the redirect Location's query string agree.
type fakeURLGenerator struct{}

func (fakeURLGenerator) LoginURL(state string) string {
	return "https://accounts.google.com/o/oauth2/auth?state=" + state
}

func TestLoginHandler_RedirectsAndSetsStateCookie(t *testing.T) {
	h := NewLoginHandler(fakeURLGenerator{}, false)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)

	location := rec.Header().Get("Location")
	require.Contains(t, location, "https://accounts.google.com/o/oauth2/auth?state=")

	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == oauthStateCookie {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie, "LoginHandler must set an oauth_state cookie")
	require.True(t, stateCookie.HttpOnly)
	require.NotEmpty(t, stateCookie.Value)
	require.Contains(t, location, stateCookie.Value, "the redirect URL's state must match the cookie's state value")
}

func TestLoginHandler_CookieSecureFlagMatchesConfig(t *testing.T) {
	h := NewLoginHandler(fakeURLGenerator{}, true)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	var stateCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == oauthStateCookie {
			stateCookie = c
		}
	}
	require.NotNil(t, stateCookie)
	require.True(t, stateCookie.Secure)
}
