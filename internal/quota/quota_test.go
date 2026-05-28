package quota

import (
	"testing"
)

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	if l.MaxAllocationsPerUser != 10 {
		t.Errorf("expected 10, got %d", l.MaxAllocationsPerUser)
	}
	if l.MaxAllocationsPerIP != 5 {
		t.Errorf("expected 5, got %d", l.MaxAllocationsPerIP)
	}
	if l.MaxBandwidthPerUser != 1024*1024 {
		t.Errorf("expected 1MB/s, got %d", l.MaxBandwidthPerUser)
	}
	if l.MaxCredentialRequestsPerMinute != 60 {
		t.Errorf("expected 60, got %d", l.MaxCredentialRequestsPerMinute)
	}
}

func TestAllowAllocation(t *testing.T) {
	tr := NewTracker(Limits{
		MaxAllocationsPerUser:          10,
		MaxAllocationsPerIP:            20, // high enough not to interfere
		MaxCredentialRequestsPerMinute: 60,
	})

	// Allow up to per-user limit.
	for i := 0; i < 10; i++ {
		if !tr.AllowAllocation("user1", "10.0.0.1") {
			t.Errorf("expected allow for allocation %d", i+1)
		}
	}

	// 11th should be denied (per-user limit).
	if tr.AllowAllocation("user1", "10.0.0.1") {
		t.Error("expected deny for 11th allocation per user")
	}
}

func TestAllowAllocationPerIP(t *testing.T) {
	tr := NewTracker(Limits{
		MaxAllocationsPerUser: 100,
		MaxAllocationsPerIP:   3,
	})

	// Users from same IP share the IP limit.
	tr.AllowAllocation("user1", "10.0.0.1")
	tr.AllowAllocation("user2", "10.0.0.1")
	tr.AllowAllocation("user3", "10.0.0.1")

	if tr.AllowAllocation("user4", "10.0.0.1") {
		t.Error("expected deny for 4th allocation per IP")
	}

	// Different IP should still be allowed.
	if !tr.AllowAllocation("user5", "10.0.0.2") {
		t.Error("expected allow for different IP")
	}
}

func TestReleaseAllocation(t *testing.T) {
	tr := NewTracker(Limits{
		MaxAllocationsPerUser: 2,
		MaxAllocationsPerIP:   10,
	})

	tr.AllowAllocation("user1", "10.0.0.1")
	tr.AllowAllocation("user1", "10.0.0.1")

	if tr.AllowAllocation("user1", "10.0.0.1") {
		t.Error("expected deny at limit")
	}

	// Release one.
	tr.ReleaseAllocation("user1", "10.0.0.1")

	if !tr.AllowAllocation("user1", "10.0.0.1") {
		t.Error("expected allow after release")
	}
}

func TestAllowCredential(t *testing.T) {
	tr := NewTracker(Limits{MaxCredentialRequestsPerMinute: 3})

	// First 3 should be allowed.
	if !tr.AllowCredential("user1") {
		t.Error("expected allow #1")
	}
	if !tr.AllowCredential("user1") {
		t.Error("expected allow #2")
	}
	if !tr.AllowCredential("user1") {
		t.Error("expected allow #3")
	}

	// 4th should be denied.
	if tr.AllowCredential("user1") {
		t.Error("expected deny #4")
	}

	// Different user should still be allowed.
	if !tr.AllowCredential("user2") {
		t.Error("expected allow for user2")
	}
}

func TestAllowBandwidth(t *testing.T) {
	tr := NewTracker(Limits{MaxBandwidthPerUser: 1000})

	if !tr.AllowBandwidth("user1", 500) {
		t.Error("expected allow 500 bytes")
	}
	if tr.AllowBandwidth("user1", 600) {
		t.Error("expected deny 600 bytes (500+600=1100 > 1000)")
	}
}

func TestAllowBandwidthUnlimited(t *testing.T) {
	tr := NewTracker(Limits{MaxBandwidthPerUser: 0})

	if !tr.AllowBandwidth("user1", 999999) {
		t.Error("expected allow with unlimited bandwidth")
	}
}

func TestActiveAllocations(t *testing.T) {
	tr := NewTracker(DefaultLimits())

	tr.AllowAllocation("user1", "10.0.0.1")
	tr.AllowAllocation("user1", "10.0.0.1")
	tr.AllowAllocation("user2", "10.0.0.2")

	if tr.ActiveAllocations() != 3 {
		t.Errorf("expected 3 active, got %d", tr.ActiveAllocations())
	}
	if tr.ActiveUsers() != 2 {
		t.Errorf("expected 2 active users, got %d", tr.ActiveUsers())
	}
}

func TestReleaseNonexistent(t *testing.T) {
	tr := NewTracker(DefaultLimits())
	// Should not panic.
	tr.ReleaseAllocation("nonexistent", "0.0.0.0")
}
