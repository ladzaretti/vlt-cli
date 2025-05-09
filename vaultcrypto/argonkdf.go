package vaultcrypto

import (
	"golang.org/x/crypto/argon2"
)

const DefaultArgon2idVersion = 19

// Argon2Params represents the parameters for the Argon2id KDF.
type Argon2Params struct {
	Memory      uint32 // Memory cost in KiB
	Time        uint32 // Time cost (iterations)
	Parallelism uint8  // Parallelism factor (number of threads)
}

type Argon2idKDF struct {
	phc    Argon2idPHC
	salt   []byte
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
		phc: Argon2idPHC{
			Argon2Params: defaultArgon2idParams,
			Version:      DefaultArgon2idVersion,
		},
		keyLen: 32,
	}

	for _, opt := range opts {
		opt(kdf)
	}

	return kdf
}

func WithSalt(salt []byte) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.salt = salt
	}
}

func WithPHC(phc Argon2idPHC) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.phc = phc
	}
}

func WithParams(params Argon2Params) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.phc.Argon2Params = params
	}
}

func WithVersion(v int) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.phc.Version = v
	}
}

func WithKeyLen(n uint32) Argon2idKDFOpt {
	return func(kdf *Argon2idKDF) {
		kdf.keyLen = n
	}
}

func (a *Argon2idKDF) Derive(password []byte) []byte {
	params := a.phc.Argon2Params
	return argon2.IDKey(password, a.salt, params.Time, params.Memory, params.Parallelism, a.keyLen)
}

func (a *Argon2idKDF) PHC() Argon2idPHC {
	return a.phc
}
