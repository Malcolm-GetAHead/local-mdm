package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/malcolm-getahead/local-mdm/internal/config"
	"github.com/malcolm-getahead/local-mdm/internal/db"
)

// Server represents the HTTP server
type Server struct {
	router *mux.Router
	db     *db.DB
	config *config.Config
	server *http.Server
}

// New creates a new API server
func New(cfg *config.Config, database *db.DB) *Server {
	s := &Server{
		router: mux.NewRouter(),
		db:     database,
		config: cfg,
	}

	s.setupRoutes()
	s.setupMiddleware()

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      s.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// API v1
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Auth routes
	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/auth/refresh", s.handleRefresh).Methods("POST")

	// Device routes (will be protected with auth middleware)
	api.HandleFunc("/devices", s.handleListDevices).Methods("GET")
	api.HandleFunc("/devices/{id}", s.handleGetDevice).Methods("GET")
	api.HandleFunc("/devices/enroll", s.handleCreateEnrollment).Methods("POST")
	api.HandleFunc("/devices/{id}", s.handleUnenrollDevice).Methods("DELETE")
	api.HandleFunc("/devices/{id}/lock", s.handleLockDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/wipe", s.handleWipeDevice).Methods("POST")

	// Policy routes
	api.HandleFunc("/policies", s.handleListPolicies).Methods("GET")
	api.HandleFunc("/policies", s.handleCreatePolicy).Methods("POST")
	api.HandleFunc("/policies/{id}", s.handleGetPolicy).Methods("GET")
	api.HandleFunc("/policies/{id}", s.handleUpdatePolicy).Methods("PUT")
	api.HandleFunc("/policies/{id}", s.handleDeletePolicy).Methods("DELETE")
	api.HandleFunc("/policies/{id}/assign", s.handleAssignPolicy).Methods("POST")

	// Platform-specific routes
	windows := s.router.PathPrefix("/windows").Subrouter()
	windows.HandleFunc("/discovery", s.handleWindowsDiscovery).Methods("GET")
	windows.HandleFunc("/enrollment", s.handleWindowsEnrollment).Methods("POST")
	windows.HandleFunc("/management", s.handleWindowsManagement).Methods("POST")

	macos := s.router.PathPrefix("/macos").Subrouter()
	macos.HandleFunc("/enroll/{token}", s.handleMacOSEnroll).Methods("GET")
	macos.HandleFunc("/checkin", s.handleMacOSCheckin).Methods("PUT")

	android := s.router.PathPrefix("/android").Subrouter()
	android.HandleFunc("/enroll/{token}/qr", s.handleAndroidQR).Methods("GET")
	android.HandleFunc("/webhook", s.handleAndroidWebhook).Methods("POST")
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	s.router.Use(loggingMiddleware)
	s.router.Use(corsMiddleware)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	fmt.Printf("Starting server on %s\n", s.server.Addr)
	
	if s.config.Server.TLS.Enabled {
		return s.server.ListenAndServeTLS(
			s.config.Server.TLS.CertFile,
			s.config.Server.TLS.KeyFile,
		)
	}
	
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Response helpers

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *ErrorInfo  `json:"error,omitempty"`
	Meta  *MetaInfo   `json:"meta,omitempty"`
}

type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type MetaInfo struct {
	Timestamp  time.Time `json:"timestamp"`
	Page       int       `json:"page,omitempty"`
	PerPage    int       `json:"per_page,omitempty"`
	Total      int       `json:"total,omitempty"`
	TotalPages int       `json:"total_pages,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	response := Response{
		Data: data,
		Meta: &MetaInfo{
			Timestamp: time.Now(),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	response := Response{
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		Meta: &MetaInfo{
			Timestamp: time.Now(),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// Middleware

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %s\n", r.Method, r.RequestURI, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
