package flydeploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.machines.dev"

// Client is an HTTP client for the Fly Machines API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	dryRun     bool
}

// NewClient creates a new Fly Machines API client.
func NewClient(token string, dryRun bool) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		token:   token,
		dryRun:  dryRun,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetBaseURL overrides the default API base URL (useful for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

// CreateApp creates a new Fly.io application.
func (c *Client) CreateApp(ctx context.Context, appName, orgSlug string) (*App, error) {
	if c.dryRun {
		return &App{Name: appName, OrgSlug: orgSlug, Status: "dry-run"}, nil
	}

	reqBody := CreateAppRequest{
		AppName: appName,
		OrgSlug: orgSlug,
	}

	var app App
	if err := c.doRequest(ctx, http.MethodPost, "/v1/apps", reqBody, &app); err != nil {
		return nil, fmt.Errorf("create app %q: %w", appName, err)
	}
	return &app, nil
}

// GetApp retrieves a Fly.io application by name.
func (c *Client) GetApp(ctx context.Context, appName string) (*App, error) {
	if c.dryRun {
		return &App{Name: appName, Status: "dry-run"}, nil
	}

	var app App
	path := fmt.Sprintf("/v1/apps/%s", appName)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &app); err != nil {
		return nil, fmt.Errorf("get app %q: %w", appName, err)
	}
	return &app, nil
}

// ListApps retrieves all Fly.io applications.
func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	if c.dryRun {
		return nil, nil
	}

	var apps []App
	if err := c.doRequest(ctx, http.MethodGet, "/v1/apps", nil, &apps); err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	return apps, nil
}

// CreateMachine creates a new Fly Machine.
func (c *Client) CreateMachine(ctx context.Context, appName string, req CreateMachineRequest) (*Machine, error) {
	if c.dryRun {
		return &Machine{
			Name:   req.Name,
			Region: req.Region,
			State:  "dry-run",
			Config: req.Config,
		}, nil
	}

	var machine Machine
	path := fmt.Sprintf("/v1/apps/%s/machines", appName)
	if err := c.doRequest(ctx, http.MethodPost, path, req, &machine); err != nil {
		return nil, fmt.Errorf("create machine in app %q: %w", appName, err)
	}
	return &machine, nil
}

// ListMachines retrieves all Machines in an app.
func (c *Client) ListMachines(ctx context.Context, appName string) ([]Machine, error) {
	if c.dryRun {
		return nil, nil
	}

	var machines []Machine
	path := fmt.Sprintf("/v1/apps/%s/machines", appName)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &machines); err != nil {
		return nil, fmt.Errorf("list machines in app %q: %w", appName, err)
	}
	return machines, nil
}

// GetMachine retrieves a specific Fly Machine.
func (c *Client) GetMachine(ctx context.Context, appName, machineID string) (*Machine, error) {
	if c.dryRun {
		return &Machine{ID: machineID, State: "dry-run"}, nil
	}

	var machine Machine
	path := fmt.Sprintf("/v1/apps/%s/machines/%s", appName, machineID)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &machine); err != nil {
		return nil, fmt.Errorf("get machine %q in app %q: %w", machineID, appName, err)
	}
	return &machine, nil
}

// UpdateMachine updates an existing Fly Machine configuration.
func (c *Client) UpdateMachine(ctx context.Context, appName, machineID string, req CreateMachineRequest) (*Machine, error) {
	if c.dryRun {
		return &Machine{
			ID:     machineID,
			Name:   req.Name,
			Region: req.Region,
			State:  "dry-run",
			Config: req.Config,
		}, nil
	}

	var machine Machine
	path := fmt.Sprintf("/v1/apps/%s/machines/%s", appName, machineID)
	if err := c.doRequest(ctx, http.MethodPost, path, req, &machine); err != nil {
		return nil, fmt.Errorf("update machine %q in app %q: %w", machineID, appName, err)
	}
	return &machine, nil
}

// StartMachine starts a Fly Machine.
func (c *Client) StartMachine(ctx context.Context, appName, machineID string) error {
	if c.dryRun {
		return nil
	}

	path := fmt.Sprintf("/v1/apps/%s/machines/%s/start", appName, machineID)
	if err := c.doRequest(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("start machine %q: %w", machineID, err)
	}
	return nil
}

// StopMachine stops a Fly Machine.
func (c *Client) StopMachine(ctx context.Context, appName, machineID string) error {
	if c.dryRun {
		return nil
	}

	path := fmt.Sprintf("/v1/apps/%s/machines/%s/stop", appName, machineID)
	if err := c.doRequest(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("stop machine %q: %w", machineID, err)
	}
	return nil
}

// DestroyMachine destroys a Fly Machine.
func (c *Client) DestroyMachine(ctx context.Context, appName, machineID string) error {
	if c.dryRun {
		return nil
	}

	path := fmt.Sprintf("/v1/apps/%s/machines/%s", appName, machineID)
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("destroy machine %q: %w", machineID, err)
	}
	return nil
}

// WaitForMachine polls until a machine reaches the target state.
func (c *Client) WaitForMachine(ctx context.Context, appName, machineID, targetState string, timeout time.Duration) error {
	if c.dryRun {
		return nil
	}

	deadline := time.Now().Add(timeout)
	path := fmt.Sprintf("/v1/apps/%s/machines/%s/wait", appName, machineID)
	query := url.Values{}
	query.Set("state", targetState)
	query.Set("timeout", fmt.Sprintf("%d", int(timeout.Seconds())))

	fullPath := path + "?" + query.Encode()

	for time.Now().Before(deadline) {
		var machine Machine
		err := c.doRequest(ctx, http.MethodGet, fullPath, nil, &machine)
		if err == nil && strings.EqualFold(machine.State, targetState) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf("machine %q did not reach state %q within %v", machineID, targetState, timeout)
}

// AllocateIP allocates a public IP address for an app.
func (c *Client) AllocateIP(ctx context.Context, appName string, req AllocateIPRequest) (*IPAddress, error) {
	if c.dryRun {
		return &IPAddress{Type: req.Type, Region: req.Region, Address: "dry-run-ip"}, nil
	}

	var ip IPAddress
	path := fmt.Sprintf("/v1/apps/%s/ip_assignments", appName)
	if err := c.doRequest(ctx, http.MethodPost, path, req, &ip); err != nil {
		return nil, fmt.Errorf("allocate ip for app %q: %w", appName, err)
	}
	if ip.Type == "" {
		ip.Type = req.Type
	}
	return &ip, nil
}

// ListIPs retrieves all IP addresses allocated to an app.
func (c *Client) ListIPs(ctx context.Context, appName string) ([]IPAddress, error) {
	if c.dryRun {
		return nil, nil
	}

	var resp struct {
		IPs []IPAddress `json:"ips"`
	}
	path := fmt.Sprintf("/v1/apps/%s/ip_assignments", appName)
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list ips for app %q: %w", appName, err)
	}
	return resp.IPs, nil
}

// doRequest performs an HTTP request to the Fly API.
func (c *Client) doRequest(ctx context.Context, method, path string, body, result interface{}) error {
	u := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr APIError
		apiErr.StatusCode = resp.StatusCode
		data, _ := io.ReadAll(resp.Body)
		apiErr.Message = string(data)
		if resp.StatusCode == 401 {
			apiErr.Message = "authentication failed — check FLY_API_TOKEN"
		}
		return &apiErr
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
