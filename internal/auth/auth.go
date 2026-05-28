// Package auth provides TURN credential generation and admin API authentication.
//
// Credential format:
//
//	username = unix_expiry_timestamp:user_id
//	password = base64(hmac_sha1(shared_secret, username))
package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"time"
)

// GenerateCredentials creates short-lived TURN credentials for the given user.
// The credentials are valid for the specified duration from now.
func GenerateCredentials(sharedSecret, userID string, validity time.Duration) (username, password string) {
	expiry := time.Now().Add(validity).Unix()
	username = fmt.Sprintf("%d:%s", expiry, userID)
	password = computePassword(sharedSecret, username)
	return username, password
}

// computePassword returns the base64-encoded HMAC-SHA1 of the secret and username.
func computePassword(secret, username string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
