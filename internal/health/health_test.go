package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewService(t *testing.T) {
	s := NewService()
	if s == nil {
		t.Fatal("expected non-nil service")
	}
	if s.GetStatus() != StatusHealthy {
		t.Errorf("expected healthy status, got %s", s.GetStatus())
	}
}

func TestRegisterAndRunChecks(t *testing.T) {
	s := NewService()
	s.Register("test-check", func() (Status, string) {
		return StatusHealthy, "all good"
	})

	results := s.RunChecks()
	if len(results) != 1 {
		t.Fatalf("expected 1 check result, got %d", len(results))
	}
	if results["test-check"] != "all good" {
		t.Errorf("expected 'all good', got %q", results["test-check"])
	}
	if s.GetStatus() != StatusHealthy {
		t.Errorf("expected healthy status, got %s", s.GetStatus())
	}
}

func TestDegradedStatus(t *testing.T) {
	s := NewService()
	s.Register("check1", func() (Status, string) {
		return StatusHealthy, "ok"
	})
	s.Register("check2", func() (Status, string) {
		return StatusDegraded, "slow"
	})

	s.RunChecks()
	if s.GetStatus() != StatusDegraded {
		t.Errorf("expected degraded status, got %s", s.GetStatus())
	}
}

func TestUnhealthyStatus(t *testing.T) {
	s := NewService()
	s.Register("check1", func() (Status, string) {
		return StatusHealthy, "ok"
	})
	s.Register("check2", func() (Status, string) {
		return StatusUnhealthy, "broken"
	})

	s.RunChecks()
	if s.GetStatus() != StatusUnhealthy {
		t.Errorf("expected unhealthy status, got %s", s.GetStatus())
	}
}

func TestHealthzHandler(t *testing.T) {
	s := NewService()
	s.Register("check1", func() (Status, string) {
		return StatusHealthy, "all good"
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != string(StatusHealthy) {
		t.Errorf("expected healthy status, got %v", body["status"])
	}
}

func TestHealthzHandlerUnhealthy(t *testing.T) {
	s := NewService()
	s.Register("check1", func() (Status, string) {
		return StatusUnhealthy, "broken"
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestHealthzMethodNotAllowed(t *testing.T) {
	s := NewService()

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Result().StatusCode)
	}
}

func TestReadyzHandler(t *testing.T) {
	s := NewService()
	s.Register("check1", func() (Status, string) {
		return StatusHealthy, "ready"
	})
	s.RunChecks()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	s.ReadyzHandler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != string(StatusHealthy) {
		t.Errorf("expected healthy status, got %v", body["status"])
	}
}
