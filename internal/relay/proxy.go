package relay

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Proxy is the relay-pair proxy that forwards TURN media packets over
// a QUIC tunnel to a peer TURN server. It manages session lifecycle
// and packet forwarding between the local TURN allocation and the tunnel.
//
// This is the experimental TURN-aware relay proxy design from SCOPE.md:
// the entry TURN server terminates the TURN session and forwards relayed
// packets through the private tunnel to the exit node.
type Proxy struct {
	tunnel   *Tunnel
	sessions *Manager
	logger   *slog.Logger

	// Metrics.
	mu             sync.Mutex
	bytesIn        uint64
	bytesOut       uint64
	packetsDropped uint64
	rttSamples     []time.Duration // ring buffer of RTT measurements
}

// ProxyConfig holds configuration for the relay proxy.
type ProxyConfig struct {
	SessionTimeout time.Duration // idle session timeout (default: 5 minutes)
	GCTickInterval time.Duration // garbage collection interval (default: 30 seconds)
}

// DefaultProxyConfig returns sensible defaults.
func DefaultProxyConfig() ProxyConfig {
	return ProxyConfig{
		SessionTimeout: 5 * time.Minute,
		GCTickInterval: 30 * time.Second,
	}
}

// NewProxy creates a new relay proxy.
func NewProxy(tunnel *Tunnel, cfg ProxyConfig, logger *slog.Logger) *Proxy {
	if logger == nil {
		logger = slog.Default()
	}
	return &Proxy{
		tunnel:   tunnel,
		sessions: NewManager(cfg.SessionTimeout),
		logger:   logger.With("component", "relay-proxy"),
	}
}

// Run starts the relay proxy main loop: receive datagrams from the tunnel,
// forward them, and handle session GC. Blocks until the context is cancelled.
func (p *Proxy) Run(ctx context.Context) error {
	p.logger.Info("relay proxy starting")

	// Start GC ticker.
	gcTicker := time.NewTicker(30 * time.Second)
	defer gcTicker.Stop()

	// Datagram receive loop.
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("relay proxy shutting down")
			return p.tunnel.Close()

		case <-gcTicker.C:
			removed := p.sessions.GC()
			if removed > 0 {
				p.logger.Debug("session GC", "removed", removed)
			}

		default:
			// Non-blocking receive with short timeout so we can
			// still handle context cancellation and GC ticks.
			recvCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			data, err := p.tunnel.ReceiveDatagram(recvCtx)
			cancel()

			if err != nil {
				// Timeout is expected; continue the loop.
				if ctx.Err() != nil {
					continue
				}
				continue
			}

			if err := p.handleDatagram(data); err != nil {
				p.logger.Warn("failed to handle datagram", "error", err)
				p.mu.Lock()
				p.packetsDropped++
				p.mu.Unlock()
			}
		}
	}
}

// ForwardPacket creates a relay frame from a TURN media packet and sends it
// through the tunnel to the peer. This is called for each packet arriving
// from the local TURN allocation that should be forwarded to the peer.
func (p *Proxy) ForwardPacket(sessionID FrameID, flowID uint16, dir Direction, payload []byte) error {
	frame := &Frame{
		SessionID: sessionID,
		FlowID:    flowID,
		Direction: dir,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	data, err := frame.Encode()
	if err != nil {
		return fmt.Errorf("encode frame: %w", err)
	}

	if err := p.tunnel.SendDatagram(data); err != nil {
		return fmt.Errorf("send datagram: %w", err)
	}

	p.mu.Lock()
	p.bytesOut += uint64(len(data))
	p.mu.Unlock()

	// Update session stats.
	if s, ok := p.sessions.Get(sessionID); ok {
		s.RecordPacketOut(len(payload))
	}

	return nil
}

// handleDatagram processes an incoming relay frame from the tunnel.
func (p *Proxy) handleDatagram(data []byte) error {
	frame, err := DecodeFrame(data)
	if err != nil {
		return fmt.Errorf("decode frame: %w", err)
	}

	p.mu.Lock()
	p.bytesIn += uint64(len(data))
	p.mu.Unlock()

	// Ensure the session exists.
	s, ok := p.sessions.Get(frame.SessionID)
	if !ok {
		// Auto-create session on first packet.
		s = p.sessions.Create(frame.SessionID, p.tunnel.Peer())
		p.logger.Debug("auto-created relay session",
			"session_id", fmt.Sprintf("%x", frame.SessionID[:8]),
		)
	}
	s.RecordPacketIn(len(frame.Payload))

	// Measure RTT from frame timestamp.
	rtt := time.Since(frame.Timestamp)
	p.mu.Lock()
	p.rttSamples = append(p.rttSamples, rtt)
	if len(p.rttSamples) > 100 {
		p.rttSamples = p.rttSamples[1:]
	}
	p.mu.Unlock()

	// In a full implementation, this is where the packet would be
	// injected into the local TURN allocation for delivery to the
	// WebRTC client. For the experimental phase, we log and count.

	return nil
}

// NewSessionID generates a cryptographically random 128-bit session ID.
func NewSessionID() (FrameID, error) {
	var id FrameID
	if _, err := rand.Read(id[:]); err != nil {
		return id, fmt.Errorf("generate session id: %w", err)
	}
	return id, nil
}

// Stats returns aggregate proxy statistics.
type ProxyStats struct {
	BytesIn        uint64
	BytesOut       uint64
	PacketsDropped uint64
	SessionsActive int
	AvgRTTMs       float64
}

// GetStats returns the current proxy statistics.
func (p *Proxy) GetStats() ProxyStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := ProxyStats{
		BytesIn:        p.bytesIn,
		BytesOut:       p.bytesOut,
		PacketsDropped: p.packetsDropped,
		SessionsActive: p.sessions.Count(),
	}

	// Compute average RTT.
	if len(p.rttSamples) > 0 {
		var total time.Duration
		for _, rtt := range p.rttSamples {
			total += rtt
		}
		stats.AvgRTTMs = float64(total.Milliseconds()) / float64(len(p.rttSamples))
	}

	return stats
}
