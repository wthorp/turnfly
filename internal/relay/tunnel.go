package relay

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"

	"github.com/quic-go/quic-go"
)

// Tunnel represents a QUIC connection between two relay peers.
// It uses QUIC datagrams for media packet forwarding and QUIC streams
// for control messages (session open/close, keepalive).
type Tunnel struct {
	conn   *quic.Conn
	logger *slog.Logger

	mu   sync.RWMutex
	peer string
}

// TunnelConfig holds configuration for establishing a relay tunnel.
type TunnelConfig struct {
	// ListenAddr is the local QUIC listen address (e.g. "0.0.0.0:4443").
	ListenAddr string

	// PeerAddr is the remote relay peer address.
	PeerAddr string

	// TLSConfig provides the TLS configuration for QUIC handshake.
	// Both peers must have compatible certificates or use a shared CA.
	TLSConfig *tls.Config

	// IsServer indicates whether this side is the server (listener) or client (dialer).
	IsServer bool
}

// ListenAndServe starts a QUIC listener and accepts a single peer connection.
// In server mode, it blocks until a client connects.
// In client mode, it dials the peer address.
func (t *Tunnel) Connect(ctx context.Context, cfg TunnelConfig) error {
	if cfg.IsServer {
		return t.listen(ctx, cfg)
	}
	return t.dial(ctx, cfg)
}

func (t *Tunnel) listen(ctx context.Context, cfg TunnelConfig) error {
	listener, err := quic.ListenAddr(cfg.ListenAddr, cfg.TLSConfig, nil)
	if err != nil {
		return fmt.Errorf("quic listen %s: %w", cfg.ListenAddr, err)
	}

	t.logger.Info("relay tunnel listening", "addr", cfg.ListenAddr)

	conn, err := listener.Accept(ctx)
	if err != nil {
		listener.Close()
		return fmt.Errorf("quic accept: %w", err)
	}

	// Close the listener once we have one connection (relay-pair is point-to-point).
	listener.Close()

	t.conn = conn
	t.peer = conn.RemoteAddr().String()
	t.logger.Info("relay tunnel connected", "peer", t.peer)
	return nil
}

func (t *Tunnel) dial(ctx context.Context, cfg TunnelConfig) error {
	t.logger.Info("relay tunnel dialing", "addr", cfg.PeerAddr)

	conn, err := quic.DialAddr(ctx, cfg.PeerAddr, cfg.TLSConfig, nil)
	if err != nil {
		return fmt.Errorf("quic dial %s: %w", cfg.PeerAddr, err)
	}

	t.conn = conn
	t.peer = cfg.PeerAddr
	t.logger.Info("relay tunnel connected", "peer", t.peer)
	return nil
}

// SendDatagram sends a relay frame as a QUIC datagram.
func (t *Tunnel) SendDatagram(data []byte) error {
	if t.conn == nil {
		return fmt.Errorf("tunnel not connected")
	}
	return t.conn.SendDatagram(data)
}

// ReceiveDatagram receives a relay frame from the QUIC datagram channel.
func (t *Tunnel) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("tunnel not connected")
	}
	return t.conn.ReceiveDatagram(ctx)
}

// OpenStream opens a new bidirectional QUIC stream for control messages.
func (t *Tunnel) OpenStream(ctx context.Context) (*quic.Stream, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("tunnel not connected")
	}
	return t.conn.OpenStreamSync(ctx)
}

// AcceptStream accepts an incoming control stream from the peer.
func (t *Tunnel) AcceptStream(ctx context.Context) (*quic.Stream, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("tunnel not connected")
	}
	return t.conn.AcceptStream(ctx)
}

// Close closes the QUIC connection.
func (t *Tunnel) Close() error {
	if t.conn != nil {
		return t.conn.CloseWithError(0, "tunnel closed")
	}
	return nil
}

// Peer returns the connected peer address.
func (t *Tunnel) Peer() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.peer
}

// IsConnected returns true if the tunnel has an active QUIC connection.
func (t *Tunnel) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.conn != nil
}

// NewTunnel creates a new Tunnel with the given logger.
func NewTunnel(logger *slog.Logger) *Tunnel {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tunnel{
		logger: logger.With("component", "relay-tunnel"),
	}
}
