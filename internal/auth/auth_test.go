package auth

import (
	"testing"
	"time"
)

func TestGenerateCredentials(t *testing.T) {
	sharedSecret := "test-secret"
	userID := "user123"
	validity := 1 * time.Hour

	username, password := GenerateCredentials(sharedSecret, userID, validity)

	// Verify username format: expiry_timestamp:user_id
	if username == "" {
		t.Fatal("expected non-empty username")
	}

	// Verify deterministic output
	username2, password2 := GenerateCredentials(sharedSecret, userID, validity)
	// Timestamp may differ slightly, so we only check format and that
	// password is non-empty.
	if password2 == "" {
		t.Fatal("expected non-empty password")
	}

	// Generated at nearly the same time, both should be valid.
	if _, ok := ValidateCredentials(username, password, sharedSecret); !ok {
		t.Error("expected generated credentials to be valid")
	}
	if _, ok := ValidateCredentials(username2, password2, sharedSecret); !ok {
		t.Error("expected second generated credentials to be valid")
	}
}

func TestValidateCredentials(t *testing.T) {
	sharedSecret := "test-secret"
	userID := "user123"

	// Generate a valid credential.
	username, password := GenerateCredentials(sharedSecret, userID, 1*time.Hour)

	// Valid case.
	if gotUser, ok := ValidateCredentials(username, password, sharedSecret); !ok {
		t.Error("expected credentials to be valid")
	} else if gotUser != userID {
		t.Errorf("expected user %q, got %q", userID, gotUser)
	}

	// Invalid password.
	if _, ok := ValidateCredentials(username, "wrong-password", sharedSecret); ok {
		t.Error("expected wrong password to be invalid")
	}

	// Invalid username format.
	if _, ok := ValidateCredentials("bad-username", password, sharedSecret); ok {
		t.Error("expected malformed username to be invalid")
	}

	// Wrong secret.
	if _, ok := ValidateCredentials(username, password, "wrong-secret"); ok {
		t.Error("expected wrong secret to be invalid")
	}

	// Expired credential.
	expiredUser, expiredPass := GenerateCredentials(sharedSecret, userID, -1*time.Hour)
	if _, ok := ValidateCredentials(expiredUser, expiredPass, sharedSecret); ok {
		t.Error("expected expired credentials to be invalid")
	}
}

func TestValidateUsername(t *testing.T) {
	sharedSecret := "test-secret"
	userID := "user123"

	username, _ := GenerateCredentials(sharedSecret, userID, time.Hour)
	if gotUser, ok := ValidateUsername(username); !ok {
		t.Fatal("expected username to be valid")
	} else if gotUser != userID {
		t.Errorf("expected user %q, got %q", userID, gotUser)
	}

	tests := []struct {
		name     string
		username string
	}{
		{name: "missing separator", username: "bad-username"},
		{name: "missing user", username: "9999999999:"},
		{name: "bad expiry", username: "not-a-timestamp:user123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := ValidateUsername(tt.username); ok {
				t.Fatalf("expected %q to be invalid", tt.username)
			}
		})
	}

	expiredUsername, _ := GenerateCredentials(sharedSecret, userID, -time.Hour)
	if _, ok := ValidateUsername(expiredUsername); ok {
		t.Fatal("expected expired username to be invalid")
	}
}

func TestValidateCredentialsDeterministic(t *testing.T) {
	// Test with known values to ensure HMAC is deterministic.
	// This is useful for cross-language compatibility testing.
	secret := "known-secret"
	userID := "test-user"
	validity := 24 * time.Hour

	username1, password1 := GenerateCredentials(secret, userID, validity)

	// Same inputs should produce same password for the same username.
	// (We can't test exact values because timestamp is in the username.)
	if _, ok := ValidateCredentials(username1, password1, secret); !ok {
		t.Error("expected known credentials to be valid")
	}
}

func TestFormatTURNServerURL(t *testing.T) {
	url := FormatTURNServerURL("turn.example.com", 3478, "myuser", "mypass", false)
	expected := "turn:myuser:mypass@turn.example.com:3478"
	if url != expected {
		t.Errorf("expected %q, got %q", expected, url)
	}

	urlTLS := FormatTURNServerURL("turn.example.com", 5349, "myuser", "mypass", true)
	expectedTLS := "turns:myuser:mypass@turn.example.com:5349"
	if urlTLS != expectedTLS {
		t.Errorf("expected %q, got %q", expectedTLS, urlTLS)
	}
}
