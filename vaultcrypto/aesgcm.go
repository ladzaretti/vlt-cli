package vaultcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

var ErrNilAESGCM = errors.New("AESGCM is nil")

// AESGCM wraps an [cipher.AEAD] using AES in GCM mode.
type AESGCM struct {
	aead cipher.AEAD
}

// NewAESGCM creates a new AES-GCM cipher using the provided key.
func NewAESGCM(key []byte) (*AESGCM, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &AESGCM{aesgcm}, nil
}

// Seal encrypts the plaintext using the given nonce.
func (g *AESGCM) Seal(nonce, plaintext []byte) ([]byte, error) {
	if g == nil {
		return nil, ErrNilAESGCM
	}

	return g.aead.Seal(nil, nonce, plaintext, nil), nil
}

// Open decrypts the ciphertext using the given nonce.
func (g *AESGCM) Open(nonce, ciphertext []byte) ([]byte, error) {
	if g == nil {
		return nil, ErrNilAESGCM
	}

	return g.aead.Open(nil, nonce, ciphertext, nil)
}

// AEAD returns the underlying cipher.AEAD instance.
func (g *AESGCM) AEAD() cipher.AEAD {
	return g.aead
}
