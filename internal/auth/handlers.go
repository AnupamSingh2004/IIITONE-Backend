package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var errNoIDToken = errors.New("no id_token in oauth response")

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

type CallbackHandler struct {
	verifier      CodeVerifier
	users         UserUpserter
	allowedDomain string
	jwtSecret     string
	frontendURL   string
}

func NewCallbackHandler(v CodeVerifier, u UserUpserter, allowedDomain, jwtSecret, frontendURL string) *CallbackHandler {
	return &CallbackHandler{verifier: v, users: u, allowedDomain: allowedDomain, jwtSecret: jwtSecret, frontendURL: frontendURL}
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

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
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	http.Redirect(w, r, h.frontendURL, http.StatusFound)
}

func mustUUID() uuid.UUID { return uuid.New() }
