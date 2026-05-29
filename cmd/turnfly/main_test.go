package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nousresearch/turnfly/internal/flydeploy"
)

func TestParseRegionList(t *testing.T) {
	got, err := parseRegionList(" iad,ord ,, lhr ")
	if err != nil {
		t.Fatalf("parseRegionList() error = %v", err)
	}
	want := []string{"iad", "ord", "lhr"}
	if len(got) != len(want) {
		t.Fatalf("expected %d regions, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("region %d = %q, want %q", i, got[i], want[i])
		}
	}

	if _, err := parseRegionList(" , "); err == nil {
		t.Fatal("expected empty region list to fail")
	}
}

func TestParseEnvFlags(t *testing.T) {
	got, err := parseEnvFlags([]string{"A=1", "B=two=parts"})
	if err != nil {
		t.Fatalf("parseEnvFlags() error = %v", err)
	}
	if got["A"] != "1" || got["B"] != "two=parts" {
		t.Fatalf("unexpected env map: %#v", got)
	}

	if _, err := parseEnvFlags([]string{"missing-equals"}); err == nil {
		t.Fatal("expected invalid env flag to fail")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret, err := generateSecret(32)
	if err != nil {
		t.Fatalf("generateSecret() error = %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
}

func TestEnsureImageAppCreatesMissingApp(t *testing.T) {
	var created bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps":
			created = true
			json.NewEncoder(w).Encode(flydeploy.App{Name: "myapp", OrgSlug: "personal"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := flydeploy.NewClient("token", false)
	client.SetBaseURL(srv.URL)
	if err := ensureImageApp(context.Background(), client, "myapp", "personal"); err != nil {
		t.Fatalf("ensureImageApp() error = %v", err)
	}
	if !created {
		t.Fatal("expected app to be created")
	}
}

func TestEnsureImageAppLeavesExistingApp(t *testing.T) {
	var created bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/apps/myapp":
			json.NewEncoder(w).Encode(flydeploy.App{Name: "myapp", OrgSlug: "personal"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/apps":
			created = true
			json.NewEncoder(w).Encode(flydeploy.App{Name: "myapp", OrgSlug: "personal"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := flydeploy.NewClient("token", false)
	client.SetBaseURL(srv.URL)
	if err := ensureImageApp(context.Background(), client, "myapp", "personal"); err != nil {
		t.Fatalf("ensureImageApp() error = %v", err)
	}
	if created {
		t.Fatal("existing app should not have been created")
	}
}
