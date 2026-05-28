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
	"strings"
	"syscall"
	"time"

	"github.com/nousresearch/turnfly/internal/config"
	"github.com/nousresearch/turnfly/internal/controlapi"
	"github.com/nousresearch/turnfly/internal/flydeploy"
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
	var (
		appName string
		orgSlug string
		regions string
		image   string
		dryRun  bool
		envVars []string
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy turnfly to Fly.io",
		Long: `Deploys turnfly to Fly.io using the Fly Machines API.

Creates or converges a Fly app, allocates public IPs, and creates Machines
in the specified regions. Uses idempotent convergence: existing machines
are left alone if they match the desired state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()
			cfg.LoadFromEnv()

			if appName != "" {
				cfg.FlyAppName = appName
			}
			if orgSlug != "" {
				cfg.FlyOrg = orgSlug
			}

			if cfg.FlyAPIToken == "" {
				return fmt.Errorf("FLY_API_TOKEN is required (set via FLY_API_TOKEN env or --fly-api-token)")
			}
			if cfg.FlyAppName == "" {
				return fmt.Errorf("app name is required (set via FLY_APP_NAME env or --app)")
			}
			if cfg.FlyOrg == "" {
				return fmt.Errorf("org slug is required (set via FLY_ORG env or --org)")
			}
			if image == "" {
				return fmt.Errorf("image is required (use --image)")
			}

			regionList := strings.Split(regions, ",")
			cleanRegions := make([]string, 0, len(regionList))
			for _, r := range regionList {
				if r = strings.TrimSpace(r); r != "" {
					cleanRegions = append(cleanRegions, r)
				}
			}
			if len(cleanRegions) == 0 {
				return fmt.Errorf("at least one region is required (use --regions)")
			}

			// Build environment from --env flags.
			env := make(map[string]string)
			for _, e := range envVars {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					env[parts[0]] = parts[1]
				}
			}

			logger := setupLogger(cfg.LogLevel)

			client := flydeploy.NewClient(cfg.FlyAPIToken, dryRun)
			deployer := flydeploy.NewDeployer(client, logger)

			deployCfg := flydeploy.DeployConfig{
				AppName:     cfg.FlyAppName,
				OrgSlug:     cfg.FlyOrg,
				Regions:     cleanRegions,
				Image:       image,
				Env:         env,
				MachineName: "turnfly",
				Guest:       flydeploy.DefaultGuest(),
			}

			result, err := deployer.Deploy(cmd.Context(), deployCfg)
			if err != nil {
				return fmt.Errorf("deploy failed: %w", err)
			}

			fmt.Printf("\nDeploy complete. App: %s\n", result.App.Name)
			fmt.Printf("Regions: %s\n", strings.Join(result.Regions, ", "))
			fmt.Printf("Machines: %d\n", len(result.Machines))
			for _, m := range result.Machines {
				fmt.Printf("  %s (%s) — %s\n", m.Name, m.Region, m.State)
			}
			for _, ip := range result.IPs {
				fmt.Printf("IP: %s (%s)\n", ip.Address, ip.Type)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Fly app name (overrides FLY_APP_NAME env)")
	cmd.Flags().StringVar(&orgSlug, "org", "", "Fly organization slug (overrides FLY_ORG env)")
	cmd.Flags().StringVar(&regions, "regions", "", "Comma-separated region list (e.g. iad,ord,lhr)")
	cmd.Flags().StringVar(&image, "image", "", "Docker image reference")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Plan deployment without creating resources")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "Environment variables (KEY=VALUE, repeatable)")

	return cmd
}

func destroyCmd() *cobra.Command {
	var (
		appName string
		yesFlag bool
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy turnfly deployment",
		Long:  "Destroys all Fly Machines for the turnfly deployment on Fly.io.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()
			cfg.LoadFromEnv()

			if appName != "" {
				cfg.FlyAppName = appName
			}

			if cfg.FlyAPIToken == "" {
				return fmt.Errorf("FLY_API_TOKEN is required (set via FLY_API_TOKEN env)")
			}
			if cfg.FlyAppName == "" {
				return fmt.Errorf("app name is required (set via FLY_APP_NAME env or --app)")
			}

			if !yesFlag {
				fmt.Printf("Destroy all machines for app %q? [y/N]: ", cfg.FlyAppName)
				var response string
				fmt.Scanln(&response)
				if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
					fmt.Println("Aborted.")
					return nil
				}
			}

			logger := setupLogger(cfg.LogLevel)

			client := flydeploy.NewClient(cfg.FlyAPIToken, dryRun)
			deployer := flydeploy.NewDeployer(client, logger)

			if err := deployer.Destroy(cmd.Context(), cfg.FlyAppName); err != nil {
				return fmt.Errorf("destroy failed: %w", err)
			}

			fmt.Println("Destroy complete.")
			return nil
		},
	}

	cmd.Flags().StringVar(&appName, "app", "", "Fly app name (overrides FLY_APP_NAME env)")
	cmd.Flags().BoolVar(&yesFlag, "yes", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Plan destruction without destroying resources")

	return cmd
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

func setupLogger(logLevel string) *slog.Logger {
	level := slog.LevelInfo
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func runServeTurn(cfg config.Config) error {
	// Setup structured logging.
	logger := setupLogger(cfg.LogLevel)

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
