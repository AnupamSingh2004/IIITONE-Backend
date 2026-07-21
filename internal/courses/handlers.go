package courses

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	branch := r.URL.Query().Get("branch")
	if branch == "" {
		http.Error(w, "missing or invalid branch", http.StatusBadRequest)
		return
	}
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		http.Error(w, "missing or invalid year", http.StatusBadRequest)
		return
	}
	semester, err := strconv.Atoi(r.URL.Query().Get("semester"))
	if err != nil {
		http.Error(w, "missing or invalid semester", http.StatusBadRequest)
		return
	}

	list, err := h.repo.List(r.Context(), branch, year, semester)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
