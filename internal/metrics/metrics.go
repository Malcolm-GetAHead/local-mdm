package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all application metrics collectors.
type Metrics struct {
	// HTTP
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPActiveRequests   prometheus.Gauge

	// Database
	DBOpenConnections    prometheus.GaugeFunc
	DBIdleConnections    prometheus.GaugeFunc
	DBWaitCount          prometheus.GaugeFunc

	// Enrollment
	EnrollmentsTotal     *prometheus.CounterVec

	// Command queue
	CommandsQueued       *prometheus.CounterVec
	CommandsPending      prometheus.Gauge

	registry *prometheus.Registry
}

// New creates a new Metrics instance with all collectors registered.
func New(db *sql.DB) *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status code.",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		HTTPActiveRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "http_active_requests",
			Help: "Number of in-flight HTTP requests.",
		}),

		EnrollmentsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "enrollments_total",
			Help: "Total enrollment attempts by platform and status.",
		}, []string{"platform", "status"}),

		CommandsQueued: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "commands_queued_total",
			Help: "Total commands queued by type.",
		}, []string{"command_type"}),

		CommandsPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "commands_pending",
			Help: "Number of pending commands in the queue.",
		}),

		registry: reg,
	}

	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPActiveRequests,
		m.EnrollmentsTotal,
		m.CommandsQueued,
		m.CommandsPending,
	)

	// DB pool metrics (only if db is provided)
	if db != nil {
		m.DBOpenConnections = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_open_connections",
			Help: "Number of open database connections.",
		}, func() float64 { return float64(db.Stats().OpenConnections) })

		m.DBIdleConnections = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_idle_connections",
			Help: "Number of idle database connections.",
		}, func() float64 { return float64(db.Stats().Idle) })

		m.DBWaitCount = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "db_wait_count_total",
			Help: "Total number of connections waited for.",
		}, func() float64 { return float64(db.Stats().WaitCount) })

		reg.MustRegister(m.DBOpenConnections, m.DBIdleConnections, m.DBWaitCount)
	}

	return m
}

// Handler returns the HTTP handler for the /metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Server runs the metrics HTTP server on a separate port.
type Server struct {
	server *http.Server
	logger *slog.Logger
}

// NewServer creates a metrics server bound to the given host:port.
func NewServer(host string, port int, m *Metrics, logger *slog.Logger) *Server {
	if host == "" {
		host = "127.0.0.1"
	}
	if port == 0 {
		port = 9090
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())

	return &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", host, port),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Start starts the metrics server in the background.
func (s *Server) Start() {
	go func() {
		s.logger.Info("metrics server starting", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("metrics server error", "error", err)
		}
	}()
}

// Shutdown gracefully stops the metrics server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
