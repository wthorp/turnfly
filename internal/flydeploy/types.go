// Package flydeploy provides a client for the Fly Machines API and
// orchestration logic for deploying turnfly to Fly.io.
package flydeploy

// API types for the Fly Machines API (https://fly.io/docs/machines/api/).

// CreateAppRequest is the request body for POST /v1/apps.
type CreateAppRequest struct {
	AppName string `json:"app_name"`
	OrgSlug string `json:"org_slug"`
	Network string `json:"network,omitempty"` // e.g. "default"
}

// App represents a Fly.io application.
type App struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	OrgSlug      string       `json:"org_slug"`
	Status       string       `json:"status"`
	Network      string       `json:"network"`
	Organization Organization `json:"organization"`
}

// Organization represents the Fly organization that owns an app.
type Organization struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// EffectiveOrgSlug returns the most specific organization slug returned by
// the Fly API.
func (a App) EffectiveOrgSlug() string {
	if a.Organization.Slug != "" {
		return a.Organization.Slug
	}
	return a.OrgSlug
}

// CreateMachineRequest is the request body for POST /v1/apps/{app_name}/machines.
type CreateMachineRequest struct {
	Name   string        `json:"name,omitempty"`
	Region string        `json:"region"`
	Config MachineConfig `json:"config"`
}

// MachineConfig defines the configuration for a Fly Machine.
type MachineConfig struct {
	Image    string            `json:"image"`
	Env      map[string]string `json:"env,omitempty"`
	Services []MachineService  `json:"services,omitempty"`
	Guest    *MachineGuest     `json:"guest,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// MachineService defines a service exposed by a Fly Machine.
type MachineService struct {
	Protocol     string              `json:"protocol"` // "tcp" or "udp"
	InternalPort int                 `json:"internal_port"`
	Ports        []MachinePort       `json:"ports,omitempty"`
	Concurrency  *ServiceConcurrency `json:"concurrency,omitempty"`
}

// MachinePort defines a port mapping for a service.
type MachinePort struct {
	Port     int      `json:"port"`
	Handlers []string `json:"handlers,omitempty"` // e.g. ["tls", "http"]
}

// ServiceConcurrency defines concurrency limits.
type ServiceConcurrency struct {
	Type      string `json:"type"` // "connections" or "requests"
	HardLimit int    `json:"hard_limit"`
	SoftLimit int    `json:"soft_limit"`
}

// MachineGuest defines the VM size for a Fly Machine.
type MachineGuest struct {
	CPUKind  string `json:"cpu_kind"` // "shared" or "performance"
	CPUs     int    `json:"cpus"`
	MemoryMB int    `json:"memory_mb"`
}

// Machine represents a Fly Machine instance.
type Machine struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Region   string        `json:"region"`
	State    string        `json:"state"` // "created", "started", "stopped", "destroyed"
	Config   MachineConfig `json:"config"`
	ImageRef string        `json:"image_ref,omitempty"`
}

// IPAddress represents a Fly.io IP address allocation.
type IPAddress struct {
	ID        string `json:"id"`
	Address   string `json:"ip"`
	Type      string `json:"type"` // "v4" or "v6"
	Region    string `json:"region"`
	CreatedAt string `json:"created_at"`
}

// AllocateIPRequest is the request body for POST /v1/apps/{app_name}/ip_assignments.
type AllocateIPRequest struct {
	Type    string `json:"type"` // "v4" or "v6"
	Region  string `json:"region,omitempty"`
	OrgSlug string `json:"org_slug,omitempty"`
}

// WaitRequest is the query parameters for the wait endpoint.
type WaitRequest struct {
	State   string `json:"-"` // target state
	Timeout int    `json:"-"` // seconds
}

// APIError represents an error response from the Fly API.
type APIError struct {
	StatusCode int
	Message    string `json:"error,omitempty"`
	Details    string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}
