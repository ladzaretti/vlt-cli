package vaultcrypto

import (
	"crypto/rand"
	"io"
)

// RandBytes generates a slice of cryptographically secure
// random bytes of the specified length.
func RandBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}

	return b, nil
}
