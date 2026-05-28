package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.TURNPort != 3478 {
		t.Errorf("expected TURNPort 3478, got %d", cfg.TURNPort)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected HTTPPort 8080, got %d", cfg.HTTPPort)
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("expected MetricsAddr :9090, got %s", cfg.MetricsAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel info, got %s", cfg.LogLevel)
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore environment.
	saveEnv := func() map[string]string {
		keys := []string{"TURN_PORT", "TURN_REALM", "TURN_SHARED_SECRET", "HTTP_PORT",
			"ADMIN_API_TOKEN", "METRICS_ADDR", "FLY_APP_NAME", "FLY_ORG",
			"RELAY_MODE", "RELAY_PEERS", "LOG_LEVEL"}
		restore := make(map[string]string)
		for _, k := range keys {
			restore[k] = os.Getenv(k)
		}
		return restore
	}
	restore := saveEnv()
	defer func() {
		for k, v := range restore {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()
	for _, k := range []string{"TURN_PORT", "TURN_REALM", "TURN_SHARED_SECRET", "HTTP_PORT",
		"ADMIN_API_TOKEN", "METRICS_ADDR", "FLY_APP_NAME", "FLY_ORG",
		"RELAY_MODE", "RELAY_PEERS", "LOG_LEVEL"} {
		os.Unsetenv(k)
	}

	os.Setenv("TURN_PORT", "1234")
	os.Setenv("TURN_REALM", "test.local")
	os.Setenv("TURN_SHARED_SECRET", "secret123")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("ADMIN_API_TOKEN", "admin123")
	os.Setenv("METRICS_ADDR", ":2112")
	os.Setenv("FLY_APP_NAME", "myapp")
	os.Setenv("FLY_ORG", "myorg")
	os.Setenv("RELAY_MODE", "true")
	os.Setenv("RELAY_PEERS", "peer1,peer2")
	os.Setenv("LOG_LEVEL", "debug")

	cfg := DefaultConfig()
	cfg.LoadFromEnv()

	if cfg.TURNPort != 1234 {
		t.Errorf("expected TURNPort 1234, got %d", cfg.TURNPort)
	}
	if cfg.TURNRealm != "test.local" {
		t.Errorf("expected TURNRealm test.local, got %s", cfg.TURNRealm)
	}
	if cfg.TURNSharedSecret != "secret123" {
		t.Errorf("expected TURNSharedSecret secret123, got %s", cfg.TURNSharedSecret)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("expected HTTPPort 9090, got %d", cfg.HTTPPort)
	}
	if cfg.AdminAPIToken != "admin123" {
		t.Errorf("expected AdminAPIToken admin123, got %s", cfg.AdminAPIToken)
	}
	if cfg.MetricsAddr != ":2112" {
		t.Errorf("expected MetricsAddr :2112, got %s", cfg.MetricsAddr)
	}
	if cfg.FlyAppName != "myapp" {
		t.Errorf("expected FlyAppName myapp, got %s", cfg.FlyAppName)
	}
	if cfg.FlyOrg != "myorg" {
		t.Errorf("expected FlyOrg myorg, got %s", cfg.FlyOrg)
	}
	if !cfg.RelayMode {
		t.Error("expected RelayMode true")
	}
	if len(cfg.RelayPeers) != 2 || cfg.RelayPeers[0] != "peer1" || cfg.RelayPeers[1] != "peer2" {
		t.Errorf("expected RelayPeers [peer1 peer2], got %v", cfg.RelayPeers)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel debug, got %s", cfg.LogLevel)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         8080,
				TURNRealm:        "myrealm",
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "info",
				MetricsAddr:      ":9090",
			},
			wantErr: false,
		},
		{
			name: "missing realm",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         8080,
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "missing shared secret",
			cfg: Config{
				TURNPort:      3478,
				HTTPPort:      8080,
				TURNRealm:     "realm",
				AdminAPIToken: "token",
				LogLevel:      "info",
			},
			wantErr: true,
		},
		{
			name: "missing admin token",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         8080,
				TURNRealm:        "realm",
				TURNSharedSecret: "secret",
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid TURN port",
			cfg: Config{
				TURNPort:         0,
				HTTPPort:         8080,
				TURNRealm:        "realm",
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid HTTP port",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         99999,
				TURNRealm:        "realm",
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "ports conflict",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         3478,
				TURNRealm:        "realm",
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "info",
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				TURNPort:         3478,
				HTTPPort:         8080,
				TURNRealm:        "realm",
				TURNSharedSecret: "secret",
				AdminAPIToken:    "token",
				LogLevel:         "trace",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
