package metrics

import (
	"testing"
)

func TestRegister(t *testing.T) {
	// Register should not panic on first call.
	// Note: we can't test duplicate registration because MustRegister panics
	// and there's no clean way to unregister in the default registry.
	Register()
}

func TestHandler(t *testing.T) {
	h := Handler()
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}
