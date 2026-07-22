package search

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearch_InvalidCourseID_BadRequest(t *testing.T) {
	// repo is never reached once course_id fails to parse, so a nil
	// *Repository is safe here (mirrors courses.TestList_MissingOrInvalidParams_BadRequest).
	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=deadlock&course_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()

	h.Search(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
