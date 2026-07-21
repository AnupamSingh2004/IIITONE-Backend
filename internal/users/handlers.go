package users

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	minYear = 1
	maxYear = 6
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	profile, err := h.repo.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

type updateProfileRequest struct {
	Branch string `json:"branch"`
	Year   int    `json:"year"`
}

func (h *Handlers) UpdateMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Branch) == "" {
		http.Error(w, "branch is required", http.StatusBadRequest)
		return
	}
	if req.Year < minYear || req.Year > maxYear {
		http.Error(w, "year must be between 1 and 6", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateProfile(r.Context(), claims.UserID, req.Branch, req.Year); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Ban and Unban are admin-only (mounted behind RequireAdmin in the router).
func (h *Handlers) Ban(w http.ResponseWriter, r *http.Request)   { h.setStatus(w, r, "banned") }
func (h *Handlers) Unban(w http.ResponseWriter, r *http.Request) { h.setStatus(w, r, "active") }

func (h *Handlers) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	id, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	if err := h.repo.SetStatus(r.Context(), id, status); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
