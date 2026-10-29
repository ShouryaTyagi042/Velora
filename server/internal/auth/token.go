package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// newToken returns a session token: 32 random bytes (256 bits), base64url without padding so it
// fits in a header unescaped.
func newToken() string {
	b := make([]byte, 32)
	rand.Read(b) // crypto/rand.Read never returns an error
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashToken is what the sessions table stores and is searched by. SHA-256 is right here and
// wrong for passwords: a token is 256 random bits, so there is nothing to guess faster; a
// password is a few dozen bits of human choice, so the hash must be slow.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
