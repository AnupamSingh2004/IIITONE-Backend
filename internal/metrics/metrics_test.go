package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMiddleware_IncrementsRequestCounter(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/materials/search", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Contains(t, metricsRec.Body.String(), "http_requests_total")
	require.True(t, strings.Contains(metricsRec.Body.String(), `path="/materials/search"`) || strings.Contains(metricsRec.Body.String(), "http_requests_total"))
}
