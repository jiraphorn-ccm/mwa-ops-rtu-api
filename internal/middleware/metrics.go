package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed.",
	}, []string{"method", "route", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)

// Metrics records request counts and latency for Prometheus scraping.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chiRoutePattern(r)
		status := strconv.Itoa(ww.Status())
		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// MetricsHandler serves the Prometheus scrape endpoint.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func chiRoutePattern(r *http.Request) string {
	if pattern := chi.RouteContext(r.Context()).RoutePattern(); pattern != "" {
		return pattern
	}
	return r.URL.Path
}
