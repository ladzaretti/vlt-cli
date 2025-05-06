package vaultcrypto

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// b64 is the base64 encoding used for PHC-formatted strings,
// with padding omitted as required by the specification.
var b64 = base64.StdEncoding.WithPadding(base64.NoPadding)

// Argon2idPHC represents a PHC-formatted Argon2id string.
//
// https://github.com/P-H-C/phc-string-format/blob/master/phc-sf-spec.md
type Argon2idPHC struct {
	Argon2Params

	Version int
	Salt    []byte
	Hash    []byte
}

// String returns the PHC-formatted string representation.
func (a Argon2idPHC) String() string {
	phc := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s",
		a.Version, a.Memory, a.Time, a.Parallelism,
		b64.EncodeToString(a.Salt),
	)

	if len(a.Hash) > 0 {
		phc += "$" + b64.EncodeToString(a.Hash)
	}

	return phc
}

// DecodeAragon2idPHC parses a PHC-formatted Argon2id string into an [Argon2idPHC] struct.
// It returns an error if the format is invalid or any component cannot be decoded.
func DecodeAragon2idPHC(str string) (Argon2idPHC, error) {
	parts := strings.Split(str, "$")

	if len(parts) < 5 {
		return Argon2idPHC{}, fmt.Errorf("phc decode: expected at least 5 fields got: %s", str)
	}

	identifier, params, saltB64, hashB64 := parts[1], parts[3], parts[4], ""

	if identifier != "argon2id" {
		return Argon2idPHC{}, fmt.Errorf("phc decode: unsupported algorithm: %s", identifier)
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Argon2idPHC{}, fmt.Errorf("phc decode: invalid version format: %w", err)
	}

	switch version {
	case 16, 19: // supported
	default:
		return Argon2idPHC{}, fmt.Errorf("phc decode: unsupported version: %d", version)
	}

	if len(parts) > 5 {
		hashB64 = parts[5]
	}

	var (
		m, t uint32
		p    uint8
	)

	_, err := fmt.Sscanf(params, "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return Argon2idPHC{}, fmt.Errorf("phc decode: invalid parameters: %w", err)
	}

	salt, err := b64.DecodeString(saltB64)
	if err != nil {
		return Argon2idPHC{}, fmt.Errorf("phc decode: invalid salt encoding: %w", err)
	}

	var hash []byte
	if len(hashB64) > 0 {
		hash, err = b64.DecodeString(hashB64)
		if err != nil {
			return Argon2idPHC{}, fmt.Errorf("phc decode: invalid hash encoding: %w", err)
		}
	}

	return Argon2idPHC{
		Version: version,
		Argon2Params: Argon2Params{
			Memory:      m,
			Time:        t,
			Parallelism: p,
		},
		Salt: salt,
		Hash: hash,
	}, nil
}
