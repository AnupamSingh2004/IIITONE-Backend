// Package metrics provides HTTP request instrumentation via Prometheus.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus collectors used to instrument HTTP handlers.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
}

// New registers the metrics collectors against a dedicated Prometheus
// registry (rather than the global DefaultRegisterer) and returns a Metrics
// instance ready to wrap handlers. Using a private registry keeps New()
// safe to call more than once within a process — e.g. across test runs —
// without panicking on duplicate collector registration.
func New() *Metrics {
	registry := prometheus.NewRegistry()
	factory := promauto.With(registry)
	return &Metrics{
		registry: registry,
		requests: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"path", "method", "status"}),
	}
}

// Middleware wraps next, recording a request counter labeled by path,
// method, and response status text after the request completes.
//
// TODO(task-19): r.URL.Path is the raw request path, not the matched route
// pattern. Once this is wired to routes with path parameters (e.g.
// /api/materials/{id}), every distinct ID ever requested becomes its own
// permanent time series in http_requests_total — unbounded cardinality
// growth. Replace with the router's matched route pattern (chi exposes this
// via chi.RouteContext(r.Context()).RoutePattern()) before wiring this
// middleware to parameterized routes.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		m.requests.WithLabelValues(r.URL.Path, r.Method, http.StatusText(sw.status)).Inc()
	})
}

// Handler returns the HTTP handler that exposes collected metrics, intended
// to be mounted at /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// statusWriter wraps http.ResponseWriter to capture the status code written
// by the handler for use in metric labels.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
