package materials

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const presignedURLExpiry = 15 * time.Minute

type detailRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (MaterialDetail, error)
}

// urlSigner is the one storage.Store capability DetailHandler needs, kept
// as its own small interface (rather than depending on the full
// storage.Store) so unit tests don't need a complete fake storage backend.
type urlSigner interface {
	PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type DetailHandler struct {
	repo  detailRepo
	store urlSigner
}

func NewDetailHandler(repo detailRepo, store urlSigner) *DetailHandler {
	return &DetailHandler{repo: repo, store: store}
}

// detailResponse matches the frontend's MaterialDetail interface exactly
// (see iiitone-web's src/app/app/materials/[id]/page.tsx).
type detailResponse struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	CourseName string    `json:"courseName"`
	FileURL    string    `json:"fileUrl"`
}

func (h *DetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "materialID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	detail, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	fileURL, err := h.store.PresignedGetURL(r.Context(), detail.FileKey, presignedURLExpiry)
	if err != nil {
		http.Error(w, "failed to generate file url", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detailResponse{
		ID: detail.ID, Title: detail.Title, Type: detail.Type,
		CourseName: detail.CourseName, FileURL: fileURL,
	})
}
