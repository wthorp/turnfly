package main

import "testing"

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
