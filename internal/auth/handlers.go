package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var errNoIDToken = errors.New("no id_token in oauth response")

// oauthStateCookie carries the anti-CSRF state value from LoginHandler's
// redirect through to CallbackHandler, which must see it match the `state`
// query param Google echoes back before trusting the callback at all.
const oauthStateCookie = "oauth_state"

type UpsertedUser struct {
	ID     uuid.UUID
	Role   string
	Status string
}

// UserUpserter persists/loads the user record for a verified identity.
// Implemented by the users package's repository (Task 11).
type UserUpserter interface {
	UpsertFromIdentity(ctx context.Context, id Identity) (UpsertedUser, error)
}

// urlGenerator is implemented by GoogleVerifier; kept as its own small
// interface (rather than reusing CodeVerifier) so LoginHandler only depends
// on the one capability it actually needs.
type urlGenerator interface {
	LoginURL(state string) string
}

// LoginHandler starts the Google OAuth flow: generates a random state
// value, stores it in a short-lived cookie, and redirects to Google's
// consent screen. CallbackHandler verifies the returned state matches.
type LoginHandler struct {
	verifier     urlGenerator
	cookieSecure bool
}

func NewLoginHandler(v urlGenerator, cookieSecure bool) *LoginHandler {
	return &LoginHandler{verifier: v, cookieSecure: cookieSecure}
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(raw)

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/auth/google",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   5 * 60,
	})

	http.Redirect(w, r, h.verifier.LoginURL(state), http.StatusFound)
}

type CallbackHandler struct {
	verifier      CodeVerifier
	users         UserUpserter
	allowedDomain string
	jwtSecret     string
	frontendURL   string
	cookieSecure  bool
}

func NewCallbackHandler(v CodeVerifier, u UserUpserter, allowedDomain, jwtSecret, frontendURL string, cookieSecure bool) *CallbackHandler {
	return &CallbackHandler{verifier: v, users: u, allowedDomain: allowedDomain, jwtSecret: jwtSecret, frontendURL: frontendURL, cookieSecure: cookieSecure}
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// Consume the state cookie now that it's been checked, regardless of
	// how the rest of the flow turns out — it's single-use.
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookie, Value: "", Path: "/auth/google", MaxAge: -1,
	})

	identity, err := h.verifier.VerifyCode(r.Context(), code)
	if err != nil {
		http.Error(w, "oauth verification failed", http.StatusBadGateway)
		return
	}

	if !ValidateCollegeIdentity(identity.Email, identity.HD, h.allowedDomain) {
		http.Error(w, "account is not a verified college account", http.StatusForbidden)
		return
	}

	user, err := h.users.UpsertFromIdentity(r.Context(), identity)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	if user.Status == "banned" {
		http.Error(w, "account is banned", http.StatusForbidden)
		return
	}

	token, err := IssueToken(h.jwtSecret, user.ID, user.Role, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	http.Redirect(w, r, h.frontendURL, http.StatusFound)
}
