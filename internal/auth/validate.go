// Package auth provides TURN credential generation and admin API authentication.
package auth

import (
	"crypto/hmac"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValidateCredentials checks whether the given username and password are valid
// ephemeral TURN credentials for the provided shared secret. Returns the user
// ID and true if valid, or an empty string and false otherwise.
func ValidateCredentials(username, password, sharedSecret string) (string, bool) {
	userID, ok := ValidateUsername(username)
	if !ok {
		return "", false
	}

	expectedPassword := computePassword(sharedSecret, username)
	if !hmac.Equal([]byte(password), []byte(expectedPassword)) {
		return "", false
	}

	return userID, true
}

// ValidateUsername checks that a TURN username has the expected ephemeral
// credential format and has not expired. It returns the embedded user ID when
// the username is valid.
func ValidateUsername(username string) (string, bool) {
	parts := strings.SplitN(username, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", false
	}

	expiryUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", false
	}

	if time.Now().Unix() > expiryUnix {
		return "", false
	}

	return parts[1], true
}

// FormatTURNServerURL formats a TURN server URL with credentials for WebRTC
// ICE configuration.
func FormatTURNServerURL(host string, port int, username, password string, useTLS bool) string {
	scheme := "turn"
	if useTLS {
		scheme = "turns"
	}
	return fmt.Sprintf("%s:%s:%s@%s:%d", scheme, username, password, host, port)
}
