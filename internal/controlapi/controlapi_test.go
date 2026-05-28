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

func TestDeployWithoutAuth(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/deploy", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestDeployWithInvalidToken(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/deploy", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Result().StatusCode)
	}
}

func TestDeployWithValidToken(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"regions": []string{"iad"},
		"image":   "test-image",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/deploy", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	// Currently returns 501 — server-side deploy is stub.
	if w.Result().StatusCode != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", w.Result().StatusCode)
	}
}

func TestRegionsEndpoint(t *testing.T) {
	hs := health.NewService()
	s := NewServer("test-secret", "test-token", 1*time.Hour, hs, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
	w := httptest.NewRecorder()

	s.Handler().ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	regions, ok := body["regions"]
	if !ok {
		t.Fatal("expected regions in response")
	}
	regionList, ok := regions.([]interface{})
	if !ok || len(regionList) == 0 {
		t.Fatal("expected non-empty regions list")
	}
}
