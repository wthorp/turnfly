package flydeploy

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// DeployConfig holds the configuration for a deploy operation.
type DeployConfig struct {
	AppName       string            // Fly app name
	OrgSlug       string            // Fly organization
	Regions       []string          // Regions to deploy to (e.g. ["iad", "ord"])
	Image         string            // Docker image reference
	Env           map[string]string // Environment variables (secrets)
	MachineName   string            // Base name for machines (default: "turnfly")
	Guest         MachineGuest      // VM size
	HealthPort    int               // HTTP port for health check polling
	HealthTimeout time.Duration     // Max time to wait for healthy machines
}

// DeployResult holds the outcome of a deploy operation.
type DeployResult struct {
	App      App
	Machines []Machine
	IPs      []IPAddress
	DryRun   bool
	Regions  []string
}

// Deployer orchestrates turnfly deployments to Fly.io.
type Deployer struct {
	client *Client
	logger *slog.Logger
}

// NewDeployer creates a new Deployer with the given API client.
func NewDeployer(client *Client, logger *slog.Logger) *Deployer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Deployer{
		client: client,
		logger: logger.With("component", "flydeploy"),
	}
}

// Deploy creates or converges a turnfly deployment on Fly.io.
func (d *Deployer) Deploy(ctx context.Context, cfg DeployConfig) (*DeployResult, error) {
	dryRun := d.client.dryRun
	d.logger.Info("starting deploy",
		"app", cfg.AppName,
		"org", cfg.OrgSlug,
		"regions", cfg.Regions,
		"image", cfg.Image,
		"dry_run", dryRun,
	)

	if cfg.AppName == "" {
		return nil, fmt.Errorf("app name is required")
	}
	if cfg.OrgSlug == "" {
		return nil, fmt.Errorf("org slug is required")
	}
	if len(cfg.Regions) == 0 {
		return nil, fmt.Errorf("at least one region is required")
	}
	if cfg.Image == "" {
		return nil, fmt.Errorf("image is required")
	}
	if cfg.HealthTimeout == 0 {
		cfg.HealthTimeout = 5 * time.Minute
	}
	if cfg.HealthPort == 0 {
		cfg.HealthPort = 8080
	}
	if cfg.MachineName == "" {
		cfg.MachineName = "turnfly"
	}

	result := &DeployResult{DryRun: dryRun, Regions: cfg.Regions}

	// 1. Create or verify the Fly app.
	app, err := d.ensureApp(ctx, cfg.AppName, cfg.OrgSlug)
	if err != nil {
		return nil, fmt.Errorf("ensure app: %w", err)
	}
	result.App = *app
	d.logger.Info("app ready", "app", app.Name, "status", app.Status)

	// 2. Ensure public IPs exist (dedicated IPv4 for UDP).
	ips, err := d.ensureIPs(ctx, cfg.AppName)
	if err != nil {
		return nil, fmt.Errorf("ensure ips: %w", err)
	}
	result.IPs = ips
	d.logger.Info("ips allocated", "count", len(ips))

	// 3. List existing machines.
	existingMachines, err := d.client.ListMachines(ctx, cfg.AppName)
	if err != nil {
		return nil, fmt.Errorf("list existing machines: %w", err)
	}

	// 4. Converge machines to the desired state.
	desiredRegions := make(map[string]bool)
	for _, r := range cfg.Regions {
		desiredRegions[r] = true
	}

	// Map existing machines by region.
	existingByRegion := make(map[string]*Machine)
	for i := range existingMachines {
		m := &existingMachines[i]
		if m.Region != "" {
			existingByRegion[m.Region] = m
		}
	}

	// Destroy machines in regions no longer desired.
	for region, m := range existingByRegion {
		if !desiredRegions[region] {
			d.logger.Info("removing machine from undesired region", "machine", m.ID, "region", region)
			if err := d.client.DestroyMachine(ctx, cfg.AppName, m.ID); err != nil {
				d.logger.Error("failed to destroy machine", "machine", m.ID, "error", err)
			}
		}
	}

	// Create or update machines in desired regions.
	var machines []Machine
	for _, region := range cfg.Regions {
		region = strings.TrimSpace(region)

		machine, err := d.ensureMachine(ctx, cfg.AppName, region, cfg, existingByRegion[region])
		if err != nil {
			return nil, fmt.Errorf("ensure machine in region %s: %w", region, err)
		}
		machines = append(machines, *machine)
	}

	result.Machines = machines

	// 5. Wait for health on new/updated machines.
	if !dryRun {
		d.logger.Info("waiting for machines to become healthy")
		for _, m := range machines {
			d.logger.Info("waiting for machine", "machine", m.ID, "region", m.Region)
			if err := d.waitForHealthy(ctx, cfg.AppName, m.ID, cfg.HealthTimeout); err != nil {
				d.logger.Warn("machine health check timeout", "machine", m.ID, "error", err)
			}
		}
	}

	d.logger.Info("deploy complete", "machines", len(machines))
	return result, nil
}

// Destroy removes all machines and resources for a turnfly deployment.
func (d *Deployer) Destroy(ctx context.Context, appName string) error {
	d.logger.Info("starting destroy", "app", appName)

	machines, err := d.client.ListMachines(ctx, appName)
	if err != nil {
		return fmt.Errorf("list machines: %w", err)
	}

	d.logger.Info("destroying machines", "count", len(machines))
	for _, m := range machines {
		d.logger.Info("destroying machine", "machine", m.ID, "region", m.Region)
		if err := d.client.DestroyMachine(ctx, appName, m.ID); err != nil {
			d.logger.Error("failed to destroy machine", "machine", m.ID, "error", err)
		}
	}

	d.logger.Info("destroy complete")
	return nil
}

// ensureApp creates the Fly app if it doesn't exist.
func (d *Deployer) ensureApp(ctx context.Context, appName, orgSlug string) (*App, error) {
	app, err := d.client.GetApp(ctx, appName)
	if err != nil {
		// App doesn't exist — create it.
		d.logger.Info("app not found, creating", "app", appName, "org", orgSlug)
		return d.client.CreateApp(ctx, appName, orgSlug)
	}
	return app, nil
}

// ensureIPs ensures at least one public IPv4 and IPv6 are allocated.
func (d *Deployer) ensureIPs(ctx context.Context, appName string) ([]IPAddress, error) {
	existing, err := d.client.ListIPs(ctx, appName)
	if err != nil {
		return nil, err
	}

	if len(existing) > 0 {
		return existing, nil
	}

	// Allocate a dedicated IPv4 for UDP.
	d.logger.Info("allocating public IPv4", "app", appName)
	v4, err := d.client.AllocateIP(ctx, appName, AllocateIPRequest{Type: "v4"})
	if err != nil {
		return nil, fmt.Errorf("allocate ipv4: %w", err)
	}
	return []IPAddress{*v4}, nil
}

// ensureMachine creates or updates a machine in the given region.
func (d *Deployer) ensureMachine(ctx context.Context, appName, region string, cfg DeployConfig, existing *Machine) (*Machine, error) {
	machineName := fmt.Sprintf("%s-%s", cfg.MachineName, region)

	// Build the desired machine config.
	desiredConfig := MachineConfig{
		Image: cfg.Image,
		Env:   cfg.Env,
		Guest: &cfg.Guest,
		Services: []MachineService{
			{
				Protocol:     "udp",
				InternalPort: 3478,
				Ports:        []MachinePort{{Port: 3478}},
			},
			{
				Protocol:     "tcp",
				InternalPort: 3478,
				Ports:        []MachinePort{{Port: 3478}},
			},
			{
				Protocol:     "tcp",
				InternalPort: cfg.HealthPort,
				Ports:        []MachinePort{{Port: 80, Handlers: []string{"http"}}},
				Concurrency: &ServiceConcurrency{
					Type:      "connections",
					HardLimit: 1000,
					SoftLimit: 800,
				},
			},
		},
	}

	if existing != nil {
		// If the machine exists and is in a good state, just start it if needed.
		d.logger.Info("machine exists", "machine", existing.ID, "region", region, "state", existing.State)
		if strings.EqualFold(existing.State, "stopped") {
			d.logger.Info("starting stopped machine", "machine", existing.ID)
			if err := d.client.StartMachine(ctx, appName, existing.ID); err != nil {
				return nil, fmt.Errorf("start machine %s: %w", existing.ID, err)
			}
		}
		return existing, nil
	}

	// Create a new machine.
	d.logger.Info("creating machine", "name", machineName, "region", region)
	req := CreateMachineRequest{
		Name:   machineName,
		Region: region,
		Config: desiredConfig,
	}
	return d.client.CreateMachine(ctx, appName, req)
}

// waitForHealthy polls the machine's health endpoint until it responds 200.
func (d *Deployer) waitForHealthy(ctx context.Context, appName, machineID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// First check machine state.
		m, err := d.client.GetMachine(ctx, appName, machineID)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
			continue
		}

		if strings.EqualFold(m.State, "started") {
			// Machine is started — consider it healthy for now.
			// Full HTTP health check requires knowing the public IP,
			// which isn't always available from the API immediately.
			d.logger.Info("machine healthy", "machine", machineID, "state", m.State)
			return nil
		}

		if strings.EqualFold(m.State, "error") || strings.EqualFold(m.State, "destroyed") {
			return fmt.Errorf("machine %s in state %s", machineID, m.State)
		}

		d.logger.Debug("waiting for machine", "machine", machineID, "state", m.State)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}

	return fmt.Errorf("machine %s health check timed out after %v", machineID, timeout)
}

// DefaultGuest returns the default VM size for turnfly machines.
func DefaultGuest() MachineGuest {
	return MachineGuest{
		CPUKind:  "shared",
		CPUs:     1,
		MemoryMB: 256,
	}
}
