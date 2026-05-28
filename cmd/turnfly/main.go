// turnfly is a self-deploying TURN server for Fly.io.
//
// Commands:
//
//	turnfly serve-turn     Start the TURN server with control API
//	turnfly serve-relay    Start experimental relay-pair mode
//	turnfly deploy         Deploy turnfly to Fly.io
//	turnfly destroy        Destroy turnfly deployment
//	turnfly probe          Run synthetic measurement probes
//	turnfly image          Build and push Docker image
//
// The serve-turn command is the primary entrypoint for Phase 1.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nousresearch/turnfly/internal/config"
	"github.com/nousresearch/turnfly/internal/controlapi"
	"github.com/nousresearch/turnfly/internal/health"
	"github.com/nousresearch/turnfly/internal/metrics"
	"github.com/nousresearch/turnfly/internal/turnserver"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "turnfly",
		Short: "Self-deploying TURN server for Fly.io",
		Long:  "turnfly runs TURN servers on Fly.io and can deploy itself using the Fly Machines API.",
	}

	rootCmd.AddCommand(serveTurnCmd())
	rootCmd.AddCommand(serveRelayCmd())
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(destroyCmd())
	rootCmd.AddCommand(probeCmd())
	rootCmd.AddCommand(imageCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func serveTurnCmd() *cobra.Command {
	var (
		turnPort    int
		httpPort    int
		metricsAddr string
		logLevel    string
	)

	cmd := &cobra.Command{
		Use:   "serve-turn",
		Short: "Start the TURN server with control API",
		Long:  "Starts a Pion TURN server on the specified UDP port and exposes the control API on HTTP.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration.
			cfg := config.DefaultConfig()
			cfg.LoadFromEnv()

			// CLI flags override env vars.
			if cmd.Flags().Changed("turn-port") {
				cfg.TURNPort = turnPort
			}
			if cmd.Flags().Changed("http-port") {
				cfg.HTTPPort = httpPort
			}
			if cmd.Flags().Changed("metrics-addr") {
				cfg.MetricsAddr = metricsAddr
			}
			if cmd.Flags().Changed("log-level") {
				cfg.LogLevel = logLevel
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			return runServeTurn(cfg)
		},
	}

	cmd.Flags().IntVar(&turnPort, "turn-port", 0, "TURN UDP listen port (overrides TURN_PORT env)")
	cmd.Flags().IntVar(&httpPort, "http-port", 0, "HTTP control API port (overrides HTTP_PORT env)")
	cmd.Flags().StringVar(&metricsAddr, "metrics-addr", "", "Metrics listen address (overrides METRICS_ADDR env)")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error (overrides LOG_LEVEL env)")

	return cmd
}

func serveRelayCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve-relay",
		Short: "Start experimental relay-pair mode",
		Long:  "Starts turnfly in experimental relay-pair mode (not implemented in Phase 1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("serve-relay is not yet implemented (planned for Phase 4)")
		},
	}
}

func deployCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy",
		Short: "Deploy turnfly to Fly.io",
		Long:  "Deploys turnfly to Fly.io using the Fly Machines API (not implemented in Phase 1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("deploy is not yet implemented (planned for Phase 2)")
		},
	}
}

func destroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy turnfly deployment",
		Long:  "Destroys the turnfly deployment on Fly.io (not implemented in Phase 1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("destroy is not yet implemented (planned for Phase 2)")
		},
	}
}

func probeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "probe",
		Short: "Run synthetic measurement probes",
		Long:  "Runs synthetic measurement probes between regions (not implemented in Phase 1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("probe is not yet implemented (planned for Phase 3)")
		},
	}
}

func imageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "image",
		Short: "Build and push Docker image",
		Long:  "Builds and pushes the turnfly Docker image (not implemented in Phase 1).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("image is not yet implemented")
		},
	}
}

func runServeTurn(cfg config.Config) error {
	// Setup structured logging.
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	logger.Info("turnfly starting",
		"turn_port", cfg.TURNPort,
		"http_port", cfg.HTTPPort,
		"realm", cfg.TURNRealm,
	)

	// Register Prometheus metrics.
	metrics.Register()

	// Setup health checks.
	healthService := health.NewService()
	healthService.Register("startup", func() (health.Status, string) {
		return health.StatusHealthy, "service is running"
	})

	// Create TURN server.
	turnCfg := turnserver.Config{
		ListenAddr:   fmt.Sprintf("0.0.0.0:%d", cfg.TURNPort),
		Realm:        cfg.TURNRealm,
		SharedSecret: cfg.TURNSharedSecret,
	}

	turnSrv, err := turnserver.New(turnCfg, logger)
	if err != nil {
		return fmt.Errorf("create turn server: %w", err)
	}

	// Create control API.
	credValidity := 24 * time.Hour
	apiServer := controlapi.NewServer(
		cfg.TURNSharedSecret,
		cfg.AdminAPIToken,
		credValidity,
		healthService,
		logger,
	)

	// Create HTTP server for control API.
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: apiServer.Handler(),
	}

	// Setup signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Start TURN server.
	g.Go(func() error {
		return turnSrv.Start(ctx)
	})

	// Start HTTP control API.
	g.Go(func() error {
		logger.Info("control API listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})

	// Handle graceful shutdown.
	g.Go(func() error {
		<-ctx.Done()
		logger.Info("shutting down services")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		return httpSrv.Shutdown(shutdownCtx)
	})

	logger.Info("turnfly started successfully")

	if err := g.Wait(); err != nil && err != context.Canceled && err != http.ErrServerClosed {
		return err
	}

	logger.Info("turnfly stopped")
	return nil
}
