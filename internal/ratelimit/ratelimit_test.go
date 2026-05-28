package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(10, 20)
	if l == nil {
		t.Fatal("expected non-nil limiter")
	}
	if l.rate != 10 {
		t.Errorf("expected rate 10, got %f", l.rate)
	}
	if l.burst != 20 {
		t.Errorf("expected burst 20, got %d", l.burst)
	}
}

func TestAllow(t *testing.T) {
	l := NewLimiter(100, 5)

	// First burst should all be allowed.
	for i := 0; i < 5; i++ {
		if !l.Allow("test-key") {
			t.Errorf("expected allow for request %d", i+1)
		}
	}

	// 6th should be denied (burst exhausted).
	if l.Allow("test-key") {
		t.Error("expected deny after burst exhausted")
	}
}

func TestAllowDifferentKeys(t *testing.T) {
	l := NewLimiter(100, 2)

	// Each key gets its own bucket.
	if !l.Allow("key1") {
		t.Error("expected allow for key1 #1")
	}
	if !l.Allow("key1") {
		t.Error("expected allow for key1 #2")
	}
	if l.Allow("key1") {
		t.Error("expected deny for key1 #3")
	}

	// key2 should be unaffected.
	if !l.Allow("key2") {
		t.Error("expected allow for key2")
	}
}

func TestReset(t *testing.T) {
	l := NewLimiter(1, 1)

	l.Allow("test") // consume the token
	l.Reset("test")

	if !l.Allow("test") {
		t.Error("expected allow after reset")
	}
}

func TestSize(t *testing.T) {
	l := NewLimiter(10, 5)

	l.Allow("a")
	l.Allow("b")
	l.Allow("c")

	if l.Size() != 3 {
		t.Errorf("expected size 3, got %d", l.Size())
	}
}

func TestMiddleware(t *testing.T) {
	l := NewLimiter(100, 5)
	mw := Middleware(l)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First requests should pass.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 6th should be rate limited.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"

	if ip := clientIP(req); ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}

	// X-Forwarded-For takes precedence.
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	if ip := clientIP(req); ip != "203.0.113.1" {
		t.Errorf("expected 203.0.113.1, got %s", ip)
	}
}
