package metrics

import (
	"net/http"
	"net/http/httptest"
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

	// Assert the exact labeled series, not just that the metric name appears
	// somewhere — a mislabeled path/method/status would still pass a bare
	// "contains http_requests_total" check.
	require.Contains(t, metricsRec.Body.String(),
		`http_requests_total{method="GET",path="/materials/search",status="OK"} 1`)
}

func TestMiddleware_DistinguishesStatusCodes(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/materials/does-not-exist", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Contains(t, metricsRec.Body.String(),
		`http_requests_total{method="POST",path="/materials/does-not-exist",status="Not Found"} 1`)
}

func TestMiddleware_DefaultsToOKWhenWriteHeaderNeverCalled(t *testing.T) {
	m := New()
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately never call w.WriteHeader — Go's net/http implicitly
		// writes 200 on the first Write, and statusWriter must still record
		// that as OK rather than a zero value.
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Contains(t, metricsRec.Body.String(),
		`http_requests_total{method="GET",path="/health",status="OK"} 1`)
}
