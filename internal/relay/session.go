package relay

import (
	"sync"
	"time"
)

// SessionState represents the lifecycle state of a relay session.
type SessionState string

const (
	StateActive  SessionState = "active"
	StateClosing SessionState = "closing"
	StateClosed  SessionState = "closed"
)

// SessionStats holds per-session counters and timing information.
type SessionStats struct {
	PacketsIn      uint64
	PacketsOut     uint64
	BytesIn        uint64
	BytesOut       uint64
	PacketsDropped uint64
	CreatedAt      time.Time
	LastActive     time.Time
}

// Session represents a relay session between two TURN servers.
type Session struct {
	ID        FrameID
	State     SessionState
	PeerAddr  string // remote peer address for this session
	Stats     SessionStats
	CreatedAt time.Time
	Timeout   time.Duration // idle timeout before session is garbage-collected
}

// IsExpired returns true if the session has been idle longer than its timeout.
func (s *Session) IsExpired() bool {
	if s.Timeout <= 0 {
		return false
	}
	return time.Since(s.Stats.LastActive) > s.Timeout
}

// Touch updates the LastActive timestamp.
func (s *Session) Touch() {
	s.Stats.LastActive = time.Now()
}

// RecordPacketIn updates inbound packet statistics.
func (s *Session) RecordPacketIn(n int) {
	s.Stats.PacketsIn++
	s.Stats.BytesIn += uint64(n)
	s.Touch()
}

// RecordPacketOut updates outbound packet statistics.
func (s *Session) RecordPacketOut(n int) {
	s.Stats.PacketsOut++
	s.Stats.BytesOut += uint64(n)
	s.Touch()
}

// RecordDrop increments the dropped packet counter.
func (s *Session) RecordDrop() {
	s.Stats.PacketsDropped++
}

// Manager is a thread-safe registry of relay sessions.
type Manager struct {
	mu             sync.RWMutex
	sessions       map[FrameID]*Session
	defaultTimeout time.Duration
}

// NewManager creates a new session Manager.
func NewManager(defaultTimeout time.Duration) *Manager {
	return &Manager{
		sessions:       make(map[FrameID]*Session),
		defaultTimeout: defaultTimeout,
	}
}

// Create creates a new session with the given ID and peer address.
func (m *Manager) Create(id FrameID, peerAddr string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	s := &Session{
		ID:        id,
		State:     StateActive,
		PeerAddr:  peerAddr,
		CreatedAt: now,
		Timeout:   m.defaultTimeout,
		Stats: SessionStats{
			CreatedAt:  now,
			LastActive: now,
		},
	}
	m.sessions[id] = s
	return s
}

// Get retrieves a session by ID.
func (m *Manager) Get(id FrameID) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Close marks a session as closed.
func (m *Manager) Close(id FrameID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return false
	}
	s.State = StateClosed
	return true
}

// Delete removes a session from the manager.
func (m *Manager) Delete(id FrameID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// List returns all active sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// Count returns the number of active and closing sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// GC removes expired sessions. Call periodically (e.g., every 30s).
func (m *Manager) GC() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	for id, s := range m.sessions {
		if s.IsExpired() {
			delete(m.sessions, id)
			removed++
		}
	}
	return removed
}
