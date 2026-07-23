package moderation

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	materials *materials.Repository
	flags     *FlagsRepository
	store     storage.Store
}

func NewHandlers(materialsRepo *materials.Repository, flagsRepo *FlagsRepository, store storage.Store) *Handlers {
	return &Handlers{materials: materialsRepo, flags: flagsRepo, store: store}
}

func (h *Handlers) ListPending(w http.ResponseWriter, r *http.Request) {
	list, err := h.materials.ListPending(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

func (h *Handlers) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "materialID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.materials.Approve(r.Context(), id); err != nil {
		http.Error(w, "approve failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reject hard-deletes the material (row + storage object), per spec — this
// is also what frees its content_hash for resubmission.
func (h *Handlers) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "materialID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.rejectAndDelete(r.Context(), w, id)
}

func (h *Handlers) rejectAndDelete(ctx context.Context, w http.ResponseWriter, id uuid.UUID) {
	fileKey, err := h.materials.GetFileKey(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if h.store != nil {
		_ = h.store.Delete(ctx, fileKey) // best-effort; row delete below is the source of truth
	}
	if err := h.materials.Delete(ctx, id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createFlagRequest struct {
	MaterialID string `json:"material_id"`
	Reason     string `json:"reason"`
}

func (h *Handlers) CreateFlag(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req createFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	materialID, err := uuid.Parse(req.MaterialID)
	if err != nil {
		http.Error(w, "invalid material_id", http.StatusBadRequest)
		return
	}
	if _, err := h.flags.Create(r.Context(), materialID, claims.UserID, req.Reason); err != nil {
		http.Error(w, "failed to create flag", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handlers) ListOpenFlags(w http.ResponseWriter, r *http.Request) {
	list, err := h.flags.ListOpen(r.Context())
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, list)
}

// ResolveFlag rejects (hard-deletes) the flagged material and resolves the flag.
// Banning the uploader, if desired, is a separate call to the users admin-ban endpoint —
// these are independent actions per spec, not implied by each other.
func (h *Handlers) ResolveFlag(w http.ResponseWriter, r *http.Request) {
	flagID, err := uuid.Parse(chi.URLParam(r, "flagID"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if materialIDStr := r.URL.Query().Get("material_id"); materialIDStr != "" {
		materialID, err := uuid.Parse(materialIDStr)
		if err != nil {
			http.Error(w, "invalid material_id", http.StatusBadRequest)
			return
		}
		fileKey, err := h.materials.GetFileKey(r.Context(), materialID)
		if err == nil {
			if h.store != nil {
				_ = h.store.Delete(r.Context(), fileKey)
			}
			_ = h.materials.Delete(r.Context(), materialID)
		}
	}

	if err := h.flags.Resolve(r.Context(), flagID); err != nil {
		http.Error(w, "resolve failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
