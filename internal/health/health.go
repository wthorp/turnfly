// Package health provides health and readiness check endpoints.
package health

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Status represents the health status of the service.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Checker is a function that returns the health status of a component.
type Checker func() (Status, string)

// Service manages health checks for turnfly components.
type Service struct {
	mu       sync.RWMutex
	checks   map[string]Checker
	status   Status
	statusMu sync.RWMutex
}

// NewService creates a new health check service.
func NewService() *Service {
	return &Service{
		checks: make(map[string]Checker),
	}
}

// Register adds a health checker for the named component.
func (s *Service) Register(name string, check Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = check
}

// GetStatus returns the aggregate health status.
func (s *Service) GetStatus() Status {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	if s.status == "" {
		return StatusHealthy
	}
	return s.status
}

// RunChecks executes all registered health checks and updates the aggregate status.
func (s *Service) RunChecks() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make(map[string]string)
	overall := StatusHealthy

	for name, check := range s.checks {
		status, msg := check()
		results[name] = msg
		if status == StatusUnhealthy {
			overall = StatusUnhealthy
		} else if status == StatusDegraded && overall == StatusHealthy {
			overall = StatusDegraded
		}
	}

	s.statusMu.Lock()
	s.status = overall
	s.statusMu.Unlock()

	return results
}

// Handler returns an http.Handler for GET /healthz.
func (s *Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		results := s.RunChecks()
		status := s.GetStatus()

		code := http.StatusOK
		if status == StatusUnhealthy {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  status,
			"details": results,
		})
	})
}

// ReadyzHandler returns an http.Handler for GET /readyz.
func (s *Service) ReadyzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		status := s.GetStatus()

		code := http.StatusOK
		if status == StatusUnhealthy {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{
			"status": string(status),
		})
	})
}
