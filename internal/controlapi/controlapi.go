// Package controlapi provides HTTP API handlers for the turnfly control plane.
// Endpoints include credential generation, health checks, metrics, and
// deployment management (future phases).
package controlapi

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nousresearch/turnfly/internal/auth"
	"github.com/nousresearch/turnfly/internal/health"
	"github.com/nousresearch/turnfly/internal/metrics"
	"github.com/nousresearch/turnfly/internal/regions"
)

// Server holds the control API dependencies and handlers.
type Server struct {
	sharedSecret  string
	adminToken    string
	credValidity  time.Duration
	healthService *health.Service
	regionStore   *regions.Store
	logger        *slog.Logger
	mux           *http.ServeMux
}

// NewServer creates a new control API server.
// regionStore may be nil if multi-region support is not enabled.
func NewServer(sharedSecret, adminToken string, credValidity time.Duration, hs *health.Service, regionStore *regions.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		sharedSecret:  sharedSecret,
		adminToken:    adminToken,
		credValidity:  credValidity,
		healthService: hs,
		regionStore:   regionStore,
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

	// Admin-protected endpoints.
	s.mux.Handle("/v1/deploy", s.requireAdmin(s.handleDeploy))
	s.mux.HandleFunc("/v1/regions", s.handleRegions)
	s.mux.HandleFunc("/v1/ice-report", s.handleICEReport)
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

	// If multi-region is configured, include TURN URIs for all regions.
	if s.regionStore != nil && s.regionStore.Count() > 0 {
		resp.URIs = s.regionStore.GenerateMultiRegionURIs(username, password, false)
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

// deployRequest is the request body for POST /v1/deploy.
type deployRequest struct {
	Regions []string          `json:"regions"`
	Image   string            `json:"image"`
	Env     map[string]string `json:"env,omitempty"`
}

// deployResponse is the response body for POST /v1/deploy.
type deployResponse struct {
	Status   string   `json:"status"`
	App      string   `json:"app"`
	Regions  []string `json:"regions"`
	Machines int      `json:"machines"`
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Deploy orchestration via the control API is a stub for now.
	// Full implementation requires wiring the flydeploy.Deployer
	// into the Server struct, which will happen when Phase 2
	// integrates server-side deploy capability.

	writeJSONError(w, http.StatusNotImplemented, "server-side deploy not yet wired (use CLI: turnfly deploy)")
}

func (s *Server) handleRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Return deployed regions if available, otherwise well-known regions.
	wellKnown := regions.WellKnownRegions()
	result := make([]map[string]string, 0)

	if s.regionStore != nil && s.regionStore.Count() > 0 {
		for _, r := range s.regionStore.List() {
			name := wellKnown[r.Code]
			if name == "" {
				name = r.Code
			}
			result = append(result, map[string]string{
				"code": r.Code,
				"name": name,
				"host": r.Host,
			})
		}
	} else {
		// Fallback: return well-known Fly regions.
		for code, name := range wellKnown {
			result = append(result, map[string]string{
				"code": code,
				"name": name,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"regions": result,
	})
}

// iceReportRequest is the request body for POST /v1/ice-report.
type iceReportRequest struct {
	SelectedRegion  string `json:"selected_region"`
	LocalCandidate  string `json:"local_candidate,omitempty"`
	RemoteCandidate string `json:"remote_candidate,omitempty"`
	PairType        string `json:"pair_type,omitempty"` // "relay", "srflx", "host", "prflx"
	RTTMs           int    `json:"rtt_ms,omitempty"`
}

func (s *Server) handleICEReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req iceReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SelectedRegion == "" {
		writeJSONError(w, http.StatusBadRequest, "selected_region is required")
		return
	}

	// Increment region candidate wins metric.
	metrics.RegionCandidateWinsTotal.WithLabelValues(req.SelectedRegion).Inc()

	if req.PairType != "" {
		s.logger.Info("ice report",
			"selected_region", req.SelectedRegion,
			"pair_type", req.PairType,
			"rtt_ms", req.RTTMs,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "recorded",
	})
}

// requireAdmin returns middleware that checks for the admin bearer token.
func (s *Server) requireAdmin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		const bearerPrefix = "Bearer "
		if len(auth) <= len(bearerPrefix) || auth[:len(bearerPrefix)] != bearerPrefix {
			writeJSONError(w, http.StatusUnauthorized, "invalid Authorization header format")
			return
		}

		token := auth[len(bearerPrefix):]
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			writeJSONError(w, http.StatusForbidden, "invalid admin token")
			return
		}

		next(w, r)
	})
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
