// Package controlapi provides HTTP API handlers for the turnfly control plane.
// Endpoints include credential generation, health checks, metrics, and
// deployment management (future phases).
package controlapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nousresearch/turnfly/internal/auth"
	"github.com/nousresearch/turnfly/internal/health"
	"github.com/nousresearch/turnfly/internal/metrics"
)

// Server holds the control API dependencies and handlers.
type Server struct {
	sharedSecret  string
	adminToken    string
	credValidity  time.Duration
	healthService *health.Service
	logger        *slog.Logger
	mux           *http.ServeMux
}

// NewServer creates a new control API server with the given configuration.
func NewServer(sharedSecret, adminToken string, credValidity time.Duration, hs *health.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		sharedSecret:  sharedSecret,
		adminToken:    adminToken,
		credValidity:  credValidity,
		healthService: hs,
		logger:        logger.With("component", "controlapi"),
		mux:           http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/v1/credentials", s.handleCredentials)
	s.mux.Handle("/healthz", s.healthService.Handler())
	s.mux.Handle("/readyz", s.healthService.ReadyzHandler())
	s.mux.Handle("/metrics", metrics.Handler())
}

// Handler returns the HTTP handler for the control API.
func (s *Server) Handler() http.Handler {
	return withMiddleware(s.mux, s.logger)
}

// CredentialsRequest is the request body for POST /v1/credentials.
type CredentialsRequest struct {
	UserID   string `json:"user_id"`
	Validity int    `json:"validity_seconds,omitempty"` // optional, defaults to server default
}

// CredentialsResponse is the response body for POST /v1/credentials.
type CredentialsResponse struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	TTL      int      `json:"ttl_seconds"`
	URIs     []string `json:"uris,omitempty"`
}

func (s *Server) handleCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeJSONError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	validity := s.credValidity
	if req.Validity > 0 {
		validity = time.Duration(req.Validity) * time.Second
	}

	username, password := auth.GenerateCredentials(s.sharedSecret, req.UserID, validity)

	resp := CredentialsResponse{
		Username: username,
		Password: password,
		TTL:      int(validity.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// withMiddleware wraps an http.Handler with logging and panic recovery.
func withMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in http handler", "path", r.URL.Path, "panic", rec)
				writeJSONError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		logger.Info("http request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
