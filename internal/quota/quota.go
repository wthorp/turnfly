// Package quota enforces per-user and per-IP limits on TURN allocations,
// bandwidth, and credential requests. It is designed to prevent abuse and
// provide cost guardrails for the TURN service.
package quota

import (
	"sync"
	"time"

	"github.com/nousresearch/turnfly/internal/metrics"
)

// Limits defines the maximum allowed values for various quota dimensions.
type Limits struct {
	// MaxAllocationsPerUser is the maximum concurrent TURN allocations per user.
	MaxAllocationsPerUser int

	// MaxAllocationsPerIP is the maximum concurrent TURN allocations per IP.
	MaxAllocationsPerIP int

	// MaxBandwidthPerUser is the maximum bytes per second per user (0 = unlimited).
	MaxBandwidthPerUser int64

	// MaxCredentialRequestsPerMinute limits credential generation rate.
	MaxCredentialRequestsPerMinute int

	// MaxAllocationLifetime limits how long an allocation can exist.
	MaxAllocationLifetime time.Duration
}

// DefaultLimits returns conservative default quota limits.
func DefaultLimits() Limits {
	return Limits{
		MaxAllocationsPerUser:          10,
		MaxAllocationsPerIP:            5,
		MaxBandwidthPerUser:            1024 * 1024, // 1 MB/s
		MaxCredentialRequestsPerMinute: 60,
		MaxAllocationLifetime:          10 * time.Minute,
	}
}

// Tracker tracks resource usage per key (user or IP).
type Tracker struct {
	mu     sync.Mutex
	limits Limits
	users  map[string]*userQuota
	ips    map[string]*ipQuota
}

type userQuota struct {
	allocations    int
	bytesPerSecond int64
	lastCredential time.Time
	credCount      int
}

type ipQuota struct {
	allocations int
}

// NewTracker creates a new quota tracker with the given limits.
func NewTracker(limits Limits) *Tracker {
	return &Tracker{
		limits: limits,
		users:  make(map[string]*userQuota),
		ips:    make(map[string]*ipQuota),
	}
}

// AllowAllocation checks if a new TURN allocation is allowed for the given
// user and IP. Returns true and tracks the allocation if allowed.
func (t *Tracker) AllowAllocation(userID, ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check per-user limit.
	uq, ok := t.users[userID]
	if !ok {
		uq = &userQuota{}
		t.users[userID] = uq
	}
	if uq.allocations >= t.limits.MaxAllocationsPerUser {
		metrics.PacketsDroppedTotal.Inc()
		return false
	}

	// Check per-IP limit.
	iq, ok := t.ips[ip]
	if !ok {
		iq = &ipQuota{}
		t.ips[ip] = iq
	}
	if iq.allocations >= t.limits.MaxAllocationsPerIP {
		metrics.PacketsDroppedTotal.Inc()
		return false
	}

	uq.allocations++
	iq.allocations++
	return true
}

// ReleaseAllocation decrements the allocation count for a user and IP.
func (t *Tracker) ReleaseAllocation(userID, ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if uq, ok := t.users[userID]; ok {
		if uq.allocations > 0 {
			uq.allocations--
		}
	}
	if iq, ok := t.ips[ip]; ok {
		if iq.allocations > 0 {
			iq.allocations--
		}
	}
}

// AllowCredential checks if a credential generation request is allowed
// for the given user (rate-limited per minute).
func (t *Tracker) AllowCredential(userID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	uq, ok := t.users[userID]
	if !ok {
		uq = &userQuota{}
		t.users[userID] = uq
	}

	// Reset counter each minute.
	if time.Since(uq.lastCredential) > time.Minute {
		uq.credCount = 0
		uq.lastCredential = time.Now()
	}

	if uq.credCount >= t.limits.MaxCredentialRequestsPerMinute {
		return false
	}

	uq.credCount++
	if uq.lastCredential.IsZero() {
		uq.lastCredential = time.Now()
	}
	return true
}

// AllowBandwidth checks if the given byte count is within the per-user
// bandwidth limit. This is a simple instantaneous check; a production
// system would use a sliding window.
func (t *Tracker) AllowBandwidth(userID string, bytes int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	uq, ok := t.users[userID]
	if !ok {
		uq = &userQuota{}
		t.users[userID] = uq
	}

	if t.limits.MaxBandwidthPerUser <= 0 {
		return true
	}

	// Simple check: deny if recent bytes exceed limit.
	// This is intentionally coarse; production would use a proper rate tracker.
	if uq.bytesPerSecond+bytes > t.limits.MaxBandwidthPerUser {
		return false
	}

	uq.bytesPerSecond += bytes
	return true
}

// ActiveAllocations returns the total number of active allocations.
func (t *Tracker) ActiveAllocations() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := 0
	for _, uq := range t.users {
		total += uq.allocations
	}
	return total
}

// ActiveUsers returns the count of users with active allocations.
func (t *Tracker) ActiveUsers() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := 0
	for _, uq := range t.users {
		if uq.allocations > 0 {
			count++
		}
	}
	return count
}
