// Package turnserver provides Pion TURN server integration for turnfly.
// It configures and manages a TURN server with ephemeral credential auth,
// UDP relay, and Prometheus metrics.
package turnserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/nousresearch/turnfly/internal/auth"
	"github.com/nousresearch/turnfly/internal/metrics"
	"github.com/pion/turn/v2"
)

// Server wraps a Pion TURN server with turnfly-specific configuration.
type Server struct {
	cfg          Config
	sharedSecret string
	realm        string
	udpListener  net.PacketConn
	turnServer   *turn.Server
	logger       *slog.Logger
}

// Config holds TURN server configuration options.
type Config struct {
	ListenAddr   string // UDP listen address (e.g. "0.0.0.0:3478")
	Realm        string // TURN realm
	SharedSecret string // HMAC shared secret for ephemeral credentials
}

// New creates a new TURN server with the given configuration.
func New(cfg Config, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		cfg:          cfg,
		sharedSecret: cfg.SharedSecret,
		realm:        cfg.Realm,
		logger:       logger.With("component", "turnserver"),
	}

	return s, nil
}

// Start begins listening for TURN requests and starts the relay server.
// It blocks until the context is cancelled or a fatal error occurs.
func (s *Server) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("resolve udp addr %s: %w", s.cfg.ListenAddr, err)
	}

	udpListener, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", s.cfg.ListenAddr, err)
	}
	s.udpListener = udpListener
	s.logger.Info("TURN server listening", "addr", s.cfg.ListenAddr)

	turnServer, err := turn.NewServer(turn.ServerConfig{
		Realm: s.realm,
		AuthHandler: func(username, realm string, srcAddr net.Addr) ([]byte, bool) {
			// Note: the password is checked later by Pion via the
			// GenerateAuthKey callback; here we just validate the
			// username format and return the key.
			userID, ok := auth.ValidateCredentials(username, "", s.sharedSecret)
			if !ok {
				// Check username format without password validation
				// (the actual HMAC is checked via GenerateAuthKey).
				metrics.AuthFailuresTotal.Inc()
				return nil, false
			}
			_ = userID // userID is available for audit logging if needed
			return turn.GenerateAuthKey(username, s.realm, s.sharedSecret), true
		},
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: udpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorPortRange{
					RelayAddress: net.ParseIP("0.0.0.0"),
					Address:      "0.0.0.0",
					MinPort:      49152,
					MaxPort:      65535,
				},
			},
		},
	})
	if err != nil {
		udpListener.Close()
		return fmt.Errorf("create turn server: %w", err)
	}
	s.turnServer = turnServer
	s.logger.Info("TURN server started", "realm", s.realm)

	// Track allocations for metrics.
	// Pion TURN doesn't have direct allocation callbacks in v2, so we
	// register a hook through the allocation manager if available.
	// For now, metrics are updated via the control API observation.

	<-ctx.Done()
	s.logger.Info("TURN server shutting down")
	return s.Shutdown()
}

// Shutdown gracefully stops the TURN server.
func (s *Server) Shutdown() error {
	if s.turnServer != nil {
		if err := s.turnServer.Close(); err != nil {
			return fmt.Errorf("close turn server: %w", err)
		}
	}
	if s.udpListener != nil {
		if err := s.udpListener.Close(); err != nil {
			return fmt.Errorf("close udp listener: %w", err)
		}
	}
	return nil
}

// Addr returns the UDP listen address.
func (s *Server) Addr() net.Addr {
	if s.udpListener != nil {
		return s.udpListener.LocalAddr()
	}
	return nil
}
