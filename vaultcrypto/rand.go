package vaultcrypto

import (
	"crypto/rand"
	"io"
)

const (
	// SaltSize is the standard byte length for cryptographic salts.
	SaltSize = 16

	// NonceSizeGCM is the recommended byte length for nonces used with AES-GCM.
	NonceSizeGCM = 12
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
