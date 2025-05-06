package vaultcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// Argon2Params represents the parameters for the Argon2id KDF.
type Argon2Params struct {
	Memory      uint32 // Memory cost in KiB
	Time        uint32 // Time cost (iterations)
	Parallelism uint8  // Parallelism factor (number of threads)
}

type Argon2idKDF struct {
	params Argon2Params
	keyLen uint32 // keyLen is the length of the derived key in bytes
}

var defaultArgon2idParams = Argon2Params{
	Memory:      64 * 1024, // 64 MiB
	Time:        1,
	Parallelism: 4,
}

type Argon2idKDFOpt func(*Argon2idKDF)

// NewArgon2idKDF creates a new [Argon2idKDF] instance with the provided options.
// It uses the following default values:
//   - Memory: 64 MiB (64 * 1024)
//   - Time: 1 iteration
//   - Parallelism: 4 threads
//   - Key length: 32 bytes
//
// These defaults can be overridden by the available [Argon2idKDFOpt] funcs.
func NewArgon2idKDF(opts ...Argon2idKDFOpt) *Argon2idKDF {
	kdf := &Argon2idKDF{
		params: defaultArgon2idParams,
		keyLen: 32,
	}

	for _, opt := range opts {
		opt(kdf)
	}

	return kdf
}

func WithParams(params Argon2Params) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.params = params
	}
}

func WithKeyLen(n uint32) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.keyLen = n
	}
}

func (a *Argon2idKDF) DeriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, a.params.Time, a.params.Memory, a.params.Parallelism, a.keyLen)
}

func (a *Argon2idKDF) Params() Argon2Params {
	return a.params
}

// RandBytes generates a slice of cryptographically secure
// random bytes of the specified length.
func RandBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}

	return b, nil
}

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
