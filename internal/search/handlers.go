package search

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := Query{Text: r.URL.Query().Get("q")}

	if raw := r.URL.Query().Get("course_id"); raw != "" {
		courseID, err := uuid.Parse(raw)
		if err != nil {
			http.Error(w, "invalid course_id", http.StatusBadRequest)
			return
		}
		q.CourseID = &courseID
	}
	if t := r.URL.Query().Get("type"); t != "" {
		q.Type = &t
	}

	results, err := h.repo.Query(r.Context(), q)
	if err != nil {
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
