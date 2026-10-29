package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// DefaultCost is bcrypt's work factor: each +1 doubles the time. 12 is about a quarter of a
// second per hash on a laptop, which a login can afford and a GPU guessing billions of
// passwords can't. The salt is random per hash and stored inside it.
const DefaultCost = 12

// maxPasswordBytes is bcrypt's limit. Longer passwords are refused rather than silently cut:
// x/crypto returns an error for them, and two passwords sharing 72 bytes must not both work.
const maxPasswordBytes = 72

// hashPassword returns the bcrypt hash of password at the given cost.
func hashPassword(password string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(h), err
}

// checkPassword reports whether password matches hash. bcrypt compares in constant time; a
// hash is never compared with ==.
func checkPassword(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, err // a malformed stored hash: a bug or a bad row, not a wrong password
	}
}
