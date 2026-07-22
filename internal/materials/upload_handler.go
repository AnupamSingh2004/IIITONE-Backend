package materials

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/google/uuid"
)

const maxUploadSize = 50 << 20 // 50MB

var validMaterialTypes = map[string]bool{"notes": true, "pyq": true, "assignment": true}

type materialsRepo interface {
	ExistsByContentHash(ctx context.Context, hash string) (bool, error)
	Create(ctx context.Context, in CreateInput) (uuid.UUID, error)
}

// courseResolver matches courses.Repository.FindOrCreate's real signature
// (internal/courses/repository.go).
type courseResolver interface {
	FindOrCreate(ctx context.Context, name, branch string, year, semester int, createdBy *uuid.UUID) (uuid.UUID, error)
}

type UploadHandler struct {
	repo    materialsRepo
	courses courseResolver
	store   storage.Store
}

func NewUploadHandler(repo materialsRepo, courses courseResolver, store storage.Store) *UploadHandler {
	return &UploadHandler{repo: repo, courses: courses, store: store}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "file too large or invalid form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "upload-*.pdf")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), file)
	if err != nil {
		http.Error(w, "failed to read upload", http.StatusInternalServerError)
		return
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	// A short/failed read leaves the unread tail of header as zero bytes,
	// which correctly fails the magic-byte check below, so the error is
	// safe to ignore here.
	header := make([]byte, 5)
	_, _ = tmp.ReadAt(header, 0)
	if !IsPDF(header) {
		http.Error(w, "only PDF files are accepted", http.StatusBadRequest)
		return
	}

	exists, err := h.repo.ExistsByContentHash(r.Context(), hash)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "this file has already been uploaded", http.StatusConflict)
		return
	}

	// Claims are only required from this point on (course resolution and
	// the eventual Create both need the uploader's identity), so we check
	// authentication here rather than up front — this keeps the PDF/dedup
	// checks above independent of auth state, matching how the endpoint's
	// tests exercise it.
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Resolve course: prefer course_id if present and valid; otherwise fall
	// back to course_name + branch + year + semester (the frontend's
	// on-the-fly course creation path — see useUploadMaterial in iiitone-web).
	courseID, resolveErr := h.resolveCourse(r, claims.UserID)
	if resolveErr != nil {
		http.Error(w, resolveErr.Error(), http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	materialType := r.FormValue("type")
	if !validMaterialTypes[materialType] {
		http.Error(w, "type must be one of: notes, pyq, assignment", http.StatusBadRequest)
		return
	}

	text, hasLayer, _ := ExtractText(tmp.Name())

	fileKey := "materials/" + hash + ".pdf"
	if h.store != nil {
		if _, seekErr := tmp.Seek(0, io.SeekStart); seekErr != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if err := h.store.Put(r.Context(), fileKey, "application/pdf", tmp, size); err != nil {
			http.Error(w, "failed to store file", http.StatusInternalServerError)
			return
		}
	}

	id, err := h.repo.Create(r.Context(), CreateInput{
		UploaderID: claims.UserID, CourseID: courseID,
		Type: materialType, Title: title,
		FileKey: fileKey, ContentHash: hash, FileSize: size,
		HasTextLayer: hasLayer, ExtractedText: text,
	})
	if err != nil {
		http.Error(w, "failed to save material", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		ID uuid.UUID `json:"id"`
	}{ID: id})
}

// resolveCourse implements the two paths iiitone-web's useUploadMaterial hook
// can take: either an existing course_id is sent directly, or (when the user
// picked/typed a course that doesn't exist yet) course_name + branch + year +
// semester are sent for on-the-fly find-or-create.
func (h *UploadHandler) resolveCourse(r *http.Request, uploaderID uuid.UUID) (uuid.UUID, error) {
	if idStr := r.FormValue("course_id"); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return uuid.Nil, errors.New("invalid course_id")
		}
		return id, nil
	}

	courseName := strings.TrimSpace(r.FormValue("course_name"))
	if courseName == "" {
		return uuid.Nil, errors.New("course_id or course_name is required")
	}
	branch := strings.TrimSpace(r.FormValue("branch"))
	if branch == "" {
		return uuid.Nil, errors.New("branch is required")
	}

	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil {
		return uuid.Nil, errors.New("year must be a valid integer")
	}
	semester, err := strconv.Atoi(r.FormValue("semester"))
	if err != nil {
		return uuid.Nil, errors.New("semester must be a valid integer")
	}

	if h.courses == nil {
		return uuid.Nil, errors.New("course resolution is not available")
	}

	return h.courses.FindOrCreate(r.Context(), courseName, branch, year, semester, &uploaderID)
}
