package courses

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestList_MissingOrInvalidParams_BadRequest(t *testing.T) {
	// repo is never reached for these cases, so a nil *Repository is safe here.
	h := NewHandlers(nil)

	tests := []struct {
		name  string
		query string
	}{
		{"missing branch", "?year=2026&semester=3"},
		{"missing year", "?branch=CSE&semester=3"},
		{"non-numeric year", "?branch=CSE&year=abc&semester=3"},
		{"missing semester", "?branch=CSE&year=2026"},
		{"non-numeric semester", "?branch=CSE&year=2026&semester=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/courses"+tt.query, nil)
			rec := httptest.NewRecorder()

			h.List(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
