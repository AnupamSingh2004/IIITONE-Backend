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
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	semester, _ := strconv.Atoi(r.URL.Query().Get("semester"))

	list, err := h.repo.List(r.Context(), branch, year, semester)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
