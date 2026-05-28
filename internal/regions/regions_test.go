package regions

import (
	"testing"
)

func TestNewStore(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if s.Count() != 0 {
		t.Errorf("expected 0 regions, got %d", s.Count())
	}
}

func TestSetAndGet(t *testing.T) {
	s := NewStore()
	r := Region{Code: "iad", Name: "Ashburn", Host: "1.2.3.4", Port: 3478}
	s.Set(r)

	got, ok := s.Get("iad")
	if !ok {
		t.Fatal("expected region to exist")
	}
	if got.Code != "iad" {
		t.Errorf("expected iad, got %s", got.Code)
	}
	if got.Host != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", got.Host)
	}
}

func TestRemove(t *testing.T) {
	s := NewStore()
	s.Set(Region{Code: "iad"})
	s.Set(Region{Code: "ord"})

	s.Remove("iad")

	if _, ok := s.Get("iad"); ok {
		t.Error("expected iad to be removed")
	}
	if s.Count() != 1 {
		t.Errorf("expected 1 region, got %d", s.Count())
	}
}

func TestList(t *testing.T) {
	s := NewStore()
	s.Set(Region{Code: "iad"})
	s.Set(Region{Code: "ord"})
	s.Set(Region{Code: "sjc"})

	list := s.List()
	if len(list) != 3 {
		t.Errorf("expected 3 regions, got %d", len(list))
	}
}

func TestGenerateICEConfig(t *testing.T) {
	s := NewStore()
	s.Set(Region{Code: "iad", Host: "1.2.3.4", Port: 3478, TLSPort: 5349})
	s.Set(Region{Code: "ord", Host: "5.6.7.8", Port: 3478, TLSPort: 5349})

	config := s.GenerateICEConfig("testuser", "testpass", false)
	if len(config.ICEServers) != 2 {
		t.Fatalf("expected 2 ICE servers, got %d", len(config.ICEServers))
	}

	iadServer := config.ICEServers[0]
	if iadServer.Username != "testuser" {
		t.Errorf("expected testuser, got %s", iadServer.Username)
	}
	if iadServer.Credential != "testpass" {
		t.Errorf("expected testpass, got %s", iadServer.Credential)
	}
	if len(iadServer.URLs) < 1 {
		t.Fatal("expected at least one URL")
	}

	// Should contain UDP URL.
	found := false
	for _, u := range iadServer.URLs {
		if u == "turn:1.2.3.4:3478?transport=udp" || u == "turn:5.6.7.8:3478?transport=udp" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected UDP URL in server list, got %v", iadServer.URLs)
	}
}

func TestGenerateICEConfigTLS(t *testing.T) {
	s := NewStore()
	s.Set(Region{Code: "iad", Host: "1.2.3.4", Port: 3478, TLSPort: 5349})

	config := s.GenerateICEConfig("testuser", "testpass", true)
	if len(config.ICEServers) != 1 {
		t.Fatalf("expected 1 ICE server, got %d", len(config.ICEServers))
	}

	urls := config.ICEServers[0].URLs
	found := false
	for _, u := range urls {
		if u == "turns:1.2.3.4:5349?transport=udp" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TLS URL, got %v", urls)
	}
}

func TestGenerateMultiRegionURIs(t *testing.T) {
	s := NewStore()
	s.Set(Region{Code: "iad", Host: "1.2.3.4", Port: 3478, TLSPort: 5349})
	s.Set(Region{Code: "ord", Host: "5.6.7.8", Port: 3478, TLSPort: 5349})

	uris := s.GenerateMultiRegionURIs("myuser", "mypass", false)
	if len(uris) != 2 {
		t.Fatalf("expected 2 URIs, got %d", len(uris))
	}

	expectedIAD := "turn:myuser:mypass@1.2.3.4:3478"
	expectedORD := "turn:myuser:mypass@5.6.7.8:3478"

	foundIAD, foundORD := false, false
	for _, u := range uris {
		if u == expectedIAD {
			foundIAD = true
		}
		if u == expectedORD {
			foundORD = true
		}
	}
	if !foundIAD {
		t.Errorf("expected URI %s, got %v", expectedIAD, uris)
	}
	if !foundORD {
		t.Errorf("expected URI %s, got %v", expectedORD, uris)
	}
}

func TestWellKnownRegions(t *testing.T) {
	wk := WellKnownRegions()
	if len(wk) < 5 {
		t.Errorf("expected at least 5 well-known regions, got %d", len(wk))
	}
	if wk["iad"] == "" {
		t.Error("expected iad to be well-known")
	}
	if wk["nrt"] == "" {
		t.Error("expected nrt to be well-known")
	}
}

func TestEmptyStoreICEConfig(t *testing.T) {
	s := NewStore()
	config := s.GenerateICEConfig("user", "pass", false)
	if len(config.ICEServers) != 0 {
		t.Errorf("expected 0 ICE servers for empty store, got %d", len(config.ICEServers))
	}
}
