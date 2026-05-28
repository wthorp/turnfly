package flydeploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := NewClient("test-token", false)
	client.SetBaseURL(srv.URL)
	return client
}

func TestCreateApp(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/apps" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		var body CreateAppRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.AppName != "myapp" || body.OrgSlug != "myorg" {
			t.Errorf("unexpected body: %+v", body)
		}

		json.NewEncoder(w).Encode(App{
			ID:      "app-123",
			Name:    body.AppName,
			OrgSlug: body.OrgSlug,
			Status:  "created",
		})
	})

	app, err := client.CreateApp(context.Background(), "myapp", "myorg")
	if err != nil {
		t.Fatalf("CreateApp() error = %v", err)
	}
	if app.Name != "myapp" {
		t.Errorf("expected name myapp, got %s", app.Name)
	}
	if app.Status != "created" {
		t.Errorf("expected status created, got %s", app.Status)
	}
}

func TestGetApp(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/apps/myapp" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(App{
			ID:      "app-123",
			Name:    "myapp",
			OrgSlug: "myorg",
			Status:  "created",
		})
	})

	app, err := client.GetApp(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetApp() error = %v", err)
	}
	if app.Name != "myapp" {
		t.Errorf("expected name myapp, got %s", app.Name)
	}
}

func TestGetAppNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := client.GetApp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestCreateMachine(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/apps/myapp/machines" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(Machine{
			ID:     "machine-1",
			Name:   "turnfly-iad",
			Region: "iad",
			State:  "created",
		})
	})

	req := CreateMachineRequest{
		Name:   "turnfly-iad",
		Region: "iad",
		Config: MachineConfig{
			Image: "registry.fly.io/turnfly:latest",
		},
	}

	machine, err := client.CreateMachine(context.Background(), "myapp", req)
	if err != nil {
		t.Fatalf("CreateMachine() error = %v", err)
	}
	if machine.Region != "iad" {
		t.Errorf("expected region iad, got %s", machine.Region)
	}
}

func TestListMachines(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		machines := []Machine{
			{ID: "m1", Name: "turnfly-iad", Region: "iad", State: "started"},
			{ID: "m2", Name: "turnfly-ord", Region: "ord", State: "started"},
		}
		json.NewEncoder(w).Encode(machines)
	})

	machines, err := client.ListMachines(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("ListMachines() error = %v", err)
	}
	if len(machines) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(machines))
	}
	if machines[0].Region != "iad" {
		t.Errorf("expected iad, got %s", machines[0].Region)
	}
}

func TestStartStopDestroyMachine(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("start", func(t *testing.T) {
		if err := client.StartMachine(context.Background(), "myapp", "m1"); err != nil {
			t.Errorf("StartMachine() error = %v", err)
		}
	})
	t.Run("stop", func(t *testing.T) {
		if err := client.StopMachine(context.Background(), "myapp", "m1"); err != nil {
			t.Errorf("StopMachine() error = %v", err)
		}
	})
	t.Run("destroy", func(t *testing.T) {
		if err := client.DestroyMachine(context.Background(), "myapp", "m1"); err != nil {
			t.Errorf("DestroyMachine() error = %v", err)
		}
	})
}

func TestAllocateIP(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(IPAddress{
			ID:      "ip-1",
			Address: "1.2.3.4",
			Type:    "v4",
		})
	})

	ip, err := client.AllocateIP(context.Background(), "myapp", AllocateIPRequest{Type: "v4"})
	if err != nil {
		t.Fatalf("AllocateIP() error = %v", err)
	}
	if ip.Address != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ip.Address)
	}
}

func TestDryRun(t *testing.T) {
	client := NewClient("test-token", true)

	// All operations should return nil errors in dry-run mode.
	app, err := client.CreateApp(context.Background(), "myapp", "myorg")
	if err != nil {
		t.Errorf("dry-run CreateApp() error = %v", err)
	}
	if app.Status != "dry-run" {
		t.Errorf("expected dry-run status, got %s", app.Status)
	}

	machine, err := client.CreateMachine(context.Background(), "myapp", CreateMachineRequest{
		Name:   "test",
		Region: "iad",
		Config: MachineConfig{Image: "img"},
	})
	if err != nil {
		t.Errorf("dry-run CreateMachine() error = %v", err)
	}
	if machine.State != "dry-run" {
		t.Errorf("expected dry-run state, got %s", machine.State)
	}

	ip, err := client.AllocateIP(context.Background(), "myapp", AllocateIPRequest{Type: "v4"})
	if err != nil {
		t.Errorf("dry-run AllocateIP() error = %v", err)
	}
	if ip.Address != "dry-run-ip" {
		t.Errorf("expected dry-run-ip, got %s", ip.Address)
	}

	if err := client.StartMachine(context.Background(), "myapp", "m1"); err != nil {
		t.Errorf("dry-run StartMachine() error = %v", err)
	}
}

func TestAuthError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.GetApp(context.Background(), "myapp")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	// Error is wrapped by client methods; check it contains the status.
	if !contains(err.Error(), "401") && !contains(err.Error(), "authentication failed") {
		t.Errorf("expected auth-related error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
