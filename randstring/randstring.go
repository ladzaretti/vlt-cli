package randstring

import (
	"crypto/rand"
	"errors"
	"math/big"
)

var (
	ErrInvalidLength = errors.New("length must be greater than 0")
	ErrEmptyAlphabet = errors.New("alphabet must not be empty")
)

const defaultAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz~`!@#$%^&*()_-+={[}]|\\:;\"'<,>.?/"

// New returns a securely generated random string of the given length.
func New(n int) (string, error) {
	return generateRandomString(n, defaultAlphabet)
}

// NewWithAlphabet returns a securely generated random string using the provided alphabet.
func NewWithAlphabet(n int, alphabet string) (string, error) {
	return generateRandomString(n, alphabet)
}

// generateRandomString returns a cryptographically secure random string using the given alphabet.
// It will return an error if the system's secure random
// number generator fails to function correctly.
func generateRandomString(n int, alphabet string) (string, error) {
	if n <= 0 {
		return "", ErrInvalidLength
	}

	if len(alphabet) == 0 {
		return "", ErrEmptyAlphabet
	}

	ret := make([]byte, n)
	for i := range n {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}

		ret[i] = alphabet[num.Int64()]
	}

	return string(ret), nil
}
