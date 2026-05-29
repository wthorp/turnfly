package flydeploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeployerNew(t *testing.T) {
	client := NewClient("token", false)
	d := NewDeployer(client, nil)
	if d == nil {
		t.Fatal("expected non-nil deployer")
	}
}

func TestDeployDryRun(t *testing.T) {
	client := NewClient("token", true) // dry-run
	d := NewDeployer(client, nil)

	cfg := DeployConfig{
		AppName:     "myapp",
		OrgSlug:     "myorg",
		Regions:     []string{"iad", "ord"},
		Image:       "registry.fly.io/turnfly:latest",
		MachineName: "turnfly",
		Guest:       DefaultGuest(),
	}

	result, err := d.Deploy(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(result.Machines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(result.Machines))
	}
	if result.App.Name != "myapp" {
		t.Errorf("expected app name myapp, got %s", result.App.Name)
	}
}

func TestDeployValidationErrors(t *testing.T) {
	client := NewClient("token", false)
	d := NewDeployer(client, nil)

	tests := []struct {
		name    string
		cfg     DeployConfig
		wantErr string
	}{
		{
			name:    "empty app name",
			cfg:     DeployConfig{},
			wantErr: "app name is required",
		},
		{
			name: "empty org",
			cfg: DeployConfig{
				AppName: "myapp",
			},
			wantErr: "org slug is required",
		},
		{
			name: "empty regions",
			cfg: DeployConfig{
				AppName: "myapp",
				OrgSlug: "myorg",
			},
			wantErr: "at least one region is required",
		},
		{
			name: "empty image",
			cfg: DeployConfig{
				AppName: "myapp",
				OrgSlug: "myorg",
				Regions: []string{"iad"},
			},
			wantErr: "image is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := d.Deploy(context.Background(), tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestDeployCreatesAppAndMachines(t *testing.T) {
	var appCreated, ipsAllocated bool
	machineCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps":
			// List apps — not used in deploy flow, but handle cleanly.
			json.NewEncoder(w).Encode([]App{})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps":
			appCreated = true
			json.NewEncoder(w).Encode(App{ID: "app-1", Name: "myapp", Status: "created"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp":
			if appCreated {
				json.NewEncoder(w).Encode(App{ID: "app-1", Name: "myapp", Status: "created"})
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp/ip_assignments":
			// List IPs — returns existing IPs after allocation.
			if ipsAllocated {
				json.NewEncoder(w).Encode(map[string][]IPAddress{
					"ips": {{ID: "ip-1", Address: "1.2.3.4", Type: "v4"}},
				})
			} else {
				json.NewEncoder(w).Encode(map[string][]IPAddress{"ips": {}})
			}
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps/myapp/ip_assignments":
			ipsAllocated = true
			var body AllocateIPRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode allocate IP body: %v", err)
			}
			if body.OrgSlug != "myorg" {
				t.Errorf("expected org slug myorg, got %q", body.OrgSlug)
			}
			json.NewEncoder(w).Encode(IPAddress{ID: "ip-1", Address: "1.2.3.4", Type: "v4"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp/machines":
			json.NewEncoder(w).Encode([]Machine{})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps/myapp/machines":
			machineCount++
			json.NewEncoder(w).Encode(Machine{
				ID:     "m-1",
				Name:   "turnfly-iad",
				Region: "iad",
				State:  "created",
			})
		case r.URL.Path == "/v1/apps/myapp/machines/m-1":
			json.NewEncoder(w).Encode(Machine{
				ID:     "m-1",
				Name:   "turnfly-iad",
				Region: "iad",
				State:  "started",
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := NewClient("token", false)
	client.SetBaseURL(srv.URL)
	d := NewDeployer(client, nil)

	cfg := DeployConfig{
		AppName:       "myapp",
		OrgSlug:       "myorg",
		Regions:       []string{"iad"},
		Image:         "registry.fly.io/turnfly:latest",
		MachineName:   "turnfly",
		Guest:         DefaultGuest(),
		HealthTimeout: 1, // short timeout for test
	}

	result, err := d.Deploy(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}

	if !appCreated {
		t.Error("expected app to be created")
	}
	if !ipsAllocated {
		t.Error("expected IPs to be allocated")
	}
	if machineCount != 1 {
		t.Errorf("expected 1 machine created, got %d", machineCount)
	}
	if len(result.Machines) != 1 {
		t.Errorf("expected 1 machine in result, got %d", len(result.Machines))
	}
}

func TestDeployExistingApp(t *testing.T) {
	var appCreated bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp":
			json.NewEncoder(w).Encode(App{ID: "app-1", Name: "myapp", Status: "created"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps":
			appCreated = true
			json.NewEncoder(w).Encode(App{ID: "app-2", Name: "myapp", Status: "created"})
		case r.URL.Path == "/v1/apps/myapp/ip_assignments":
			json.NewEncoder(w).Encode(map[string][]IPAddress{
				"ips": {{ID: "ip-1", Address: "1.2.3.4", Type: "v4"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp/machines":
			json.NewEncoder(w).Encode([]Machine{})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps/myapp/machines":
			json.NewEncoder(w).Encode(Machine{
				ID: "m-1", Name: "turnfly-iad", Region: "iad", State: "created",
			})
		default:
			json.NewEncoder(w).Encode(Machine{ID: "m-1", State: "started"})
		}
	}))
	defer srv.Close()

	client := NewClient("token", false)
	client.SetBaseURL(srv.URL)
	d := NewDeployer(client, nil)

	cfg := DeployConfig{
		AppName:       "myapp",
		OrgSlug:       "myorg",
		Regions:       []string{"iad"},
		Image:         "registry.fly.io/turnfly:latest",
		HealthTimeout: 1,
	}

	_, err := d.Deploy(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}

	if appCreated {
		t.Error("app should not have been created (already exists)")
	}
}

func TestDeployUpdatesExistingMachine(t *testing.T) {
	var machineUpdated bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp":
			json.NewEncoder(w).Encode(App{ID: "app-1", Name: "myapp", Status: "created"})
		case r.URL.Path == "/v1/apps/myapp/ip_assignments":
			json.NewEncoder(w).Encode(map[string][]IPAddress{
				"ips": {{ID: "ip-1", Address: "1.2.3.4", Type: "v4"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp/machines":
			json.NewEncoder(w).Encode([]Machine{
				{
					ID:     "m1",
					Name:   "turnfly-iad",
					Region: "iad",
					State:  "started",
					Config: MachineConfig{Image: "registry.fly.io/myapp:old"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps/myapp/machines/m1":
			machineUpdated = true
			var body CreateMachineRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Config.Image != "registry.fly.io/myapp:new" {
				t.Errorf("expected new image, got %s", body.Config.Image)
			}
			json.NewEncoder(w).Encode(Machine{
				ID:     "m1",
				Name:   body.Name,
				Region: body.Region,
				State:  "started",
				Config: body.Config,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps/myapp/machines":
			t.Fatal("expected existing machine to be updated, not recreated")
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := NewClient("token", false)
	client.SetBaseURL(srv.URL)
	d := NewDeployer(client, nil)

	_, err := d.Deploy(context.Background(), DeployConfig{
		AppName:       "myapp",
		OrgSlug:       "myorg",
		Regions:       []string{"iad"},
		Image:         "registry.fly.io/myapp:new",
		HealthTimeout: 1,
	})
	if err != nil {
		t.Fatalf("Deploy() error = %v", err)
	}
	if !machineUpdated {
		t.Fatal("expected existing machine to be updated")
	}
}

func TestDestroy(t *testing.T) {
	machinesDestroyed := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp/machines":
			json.NewEncoder(w).Encode([]Machine{
				{ID: "m1", Name: "turnfly-iad", Region: "iad", State: "started"},
				{ID: "m2", Name: "turnfly-ord", Region: "ord", State: "started"},
			})
		case r.Method == http.MethodDelete:
			machinesDestroyed++
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := NewClient("token", false)
	client.SetBaseURL(srv.URL)
	d := NewDeployer(client, nil)

	if err := d.Destroy(context.Background(), "myapp"); err != nil {
		t.Fatalf("Destroy() error = %v", err)
	}
	if machinesDestroyed != 2 {
		t.Errorf("expected 2 machines destroyed, got %d", machinesDestroyed)
	}
}
