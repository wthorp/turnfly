// Package regions manages deployed TURN regions and generates WebRTC ICE
// server configurations for multi-region independent TURN mode.
//
// Each region represents a deployed TURN server, identified by its Fly.io
// region code and public IP address. The region set can be updated as
// deployments converge.
package regions

import (
	"fmt"
	"sync"
)

// Region represents a deployed TURN server in a Fly.io region.
type Region struct {
	Code    string `json:"code"`     // Fly region code (e.g. "iad")
	Name    string `json:"name"`     // Human-readable name
	Host    string `json:"host"`     // Public IP or hostname
	Port    int    `json:"port"`     // TURN port (typically 3478)
	TLSPort int    `json:"tls_port"` // TURN TLS port (typically 5349)
}

// ICEServer is a WebRTC ICE server entry for use in RTCPeerConnection.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

// ICEConfig holds the complete ICE server configuration for a client.
type ICEConfig struct {
	ICEServers []ICEServer `json:"iceServers"`
}

// Store is a thread-safe registry of deployed TURN regions.
type Store struct {
	mu      sync.RWMutex
	regions map[string]Region // keyed by region code
}

// NewStore creates a new empty region store.
func NewStore() *Store {
	return &Store{
		regions: make(map[string]Region),
	}
}

// Set updates or adds a region in the store.
func (s *Store) Set(r Region) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regions[r.Code] = r
}

// Remove deletes a region from the store.
func (s *Store) Remove(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.regions, code)
}

// Get returns the region for the given code, if registered.
func (s *Store) Get(code string) (Region, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.regions[code]
	return r, ok
}

// List returns all registered regions.
func (s *Store) List() []Region {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Region, 0, len(s.regions))
	for _, r := range s.regions {
		result = append(result, r)
	}
	return result
}

// Count returns the number of registered regions.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.regions)
}

// GenerateICEConfig creates a WebRTC ICE server configuration for the given
// user, covering all registered regions. Credentials are ephemeral — the
// same username/password pair works across all TURN servers because they
// share the same TURN_SHARED_SECRET.
//
// If useTLS is true, the TURN URIs use the "turns:" scheme and TLS port.
func (s *Store) GenerateICEConfig(username, password string, useTLS bool) ICEConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	servers := make([]ICEServer, 0, len(s.regions))
	for _, r := range s.regions {
		port := r.Port
		scheme := "turn"
		if useTLS {
			port = r.TLSPort
			scheme = "turns"
		}

		urls := []string{
			fmt.Sprintf("%s:%s:%d?transport=udp", scheme, r.Host, port),
		}
		// Add TCP variant as a fallback.
		urls = append(urls, fmt.Sprintf("%s:%s:%d?transport=tcp", scheme, r.Host, port))

		servers = append(servers, ICEServer{
			URLs:       urls,
			Username:   username,
			Credential: password,
		})
	}

	return ICEConfig{ICEServers: servers}
}

// GenerateMultiRegionURIs returns a flat list of TURN URIs with embedded
// credentials, suitable for the URIs field in a credentials response.
func (s *Store) GenerateMultiRegionURIs(username, password string, useTLS bool) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uris := make([]string, 0, len(s.regions)*2)
	for _, r := range s.regions {
		port := r.Port
		scheme := "turn"
		if useTLS {
			port = r.TLSPort
			scheme = "turns"
		}
		uris = append(uris,
			fmt.Sprintf("%s:%s:%s@%s:%d", scheme, username, password, r.Host, port),
		)
	}

	return uris
}

// WellKnownRegions returns a map of Fly region codes to human-readable names.
func WellKnownRegions() map[string]string {
	return map[string]string{
		"ams": "Amsterdam, Netherlands",
		"arn": "Stockholm, Sweden",
		"atl": "Atlanta, Georgia",
		"bog": "Bogotá, Colombia",
		"bos": "Boston, Massachusetts",
		"cdg": "Paris, France",
		"den": "Denver, Colorado",
		"dfw": "Dallas, Texas",
		"ewr": "Newark, New Jersey",
		"fra": "Frankfurt, Germany",
		"gdl": "Guadalajara, Mexico",
		"gru": "São Paulo, Brazil",
		"hkg": "Hong Kong",
		"iad": "Ashburn, Virginia",
		"jnb": "Johannesburg, South Africa",
		"lax": "Los Angeles, California",
		"lhr": "London, UK",
		"maa": "Chennai, India",
		"mad": "Madrid, Spain",
		"mia": "Miami, Florida",
		"nrt": "Tokyo, Japan",
		"ord": "Chicago, Illinois",
		"otp": "Bucharest, Romania",
		"phx": "Phoenix, Arizona",
		"qro": "Querétaro, Mexico",
		"scl": "Santiago, Chile",
		"sea": "Seattle, Washington",
		"sin": "Singapore",
		"sjc": "Sunnyvale, California",
		"syd": "Sydney, Australia",
		"waw": "Warsaw, Poland",
		"yul": "Montreal, Canada",
		"yyz": "Toronto, Canada",
	}
}
