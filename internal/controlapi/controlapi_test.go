package controlapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nousresearch/turnfly/internal/health"
)

func TestNewServer(t *testing.T) {
	hs := health.NewService()
	s := NewServer("secret", "token", 1*time.Hour, hs, nil)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestPostCredentials(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	body := CredentialsRequest{
		UserID:   "user123",
		Validity: 3600,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/credentials", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var credResp CredentialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&credResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if credResp.Username == "" {
		t.Error("expected non-empty username")
	}
	if credResp.Password == "" {
		t.Error("expected non-empty password")
	}
	if credResp.TTL != 3600 {
		t.Errorf("expected TTL 3600, got %d", credResp.TTL)
	}
}

func TestPostCredentialsMissingUserID(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	body := CredentialsRequest{}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/credentials", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestPostCredentialsDefaultValidity(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 24*time.Hour, hs, nil)

	body := CredentialsRequest{
		UserID: "user456",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/credentials", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var credResp CredentialsResponse
	if err := json.NewDecoder(resp.Body).Decode(&credResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if credResp.TTL != 86400 {
		t.Errorf("expected TTL 86400, got %d", credResp.TTL)
	}
}

func TestPostCredentialsMethodNotAllowed(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/credentials", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Result().StatusCode)
	}
}

func TestPostCredentialsInvalidJSON(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/credentials", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestHealthzEndpoint(t *testing.T) {
	hs := health.NewService()
	hs.Register("test", func() (health.Status, string) {
		return health.StatusHealthy, "ok"
	})
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}
}

func TestReadyzEndpoint(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}
}
