package turnserver

import (
	"testing"
)

func TestNew(t *testing.T) {
	cfg := Config{
		ListenAddr:   "0.0.0.0:3478",
		Realm:        "test.local",
		SharedSecret: "test-secret",
	}

	srv, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.realm != "test.local" {
		t.Errorf("expected realm test.local, got %s", srv.realm)
	}
	if srv.sharedSecret != "test-secret" {
		t.Errorf("expected sharedSecret test-secret, got %s", srv.sharedSecret)
	}
}

func TestAddrBeforeStart(t *testing.T) {
	cfg := Config{
		ListenAddr:   "0.0.0.0:3478",
		Realm:        "test.local",
		SharedSecret: "test-secret",
	}

	srv, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Before Start(), addr should be nil.
	if srv.Addr() != nil {
		t.Error("expected nil addr before Start()")
	}
}

func TestShutdownWithoutStart(t *testing.T) {
	cfg := Config{
		ListenAddr:   "0.0.0.0:3478",
		Realm:        "test.local",
		SharedSecret: "test-secret",
	}

	srv, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Shutdown should not panic even if server was never started.
	if err := srv.Shutdown(); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}
