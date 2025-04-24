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

const (
	lower           = "abcdefghijklmnopqrstuvwxyz"
	upper           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	symbols         = "~`!@#$%^&*()_-+={[}]|\\:;\"'<,>.?/"
	digits          = "0123456789"
	defaultAlphabet = digits + upper + lower + symbols
)

type PasswordPolicy struct {
	MinLowercase int // Minimum number of lowercase letters required.
	MinUppercase int // Minimum number of uppercase letters required.
	MinDigits    int // Minimum number of numeric digits required.
	MinSymbols   int // Minimum number of special symbols required.
	MinLength    int // Minimum total length of the password.
}

// New returns a securely generated random string of the given length.
func New(n int) (string, error) {
	return generateRandomString(n, defaultAlphabet)
}

// NewWithAlphabet returns a securely generated random string using the provided alphabet.
func NewWithAlphabet(n int, alphabet string) (string, error) {
	return generateRandomString(n, alphabet)
}

// NewWithPolicy generates a random password that satisfies the given [PasswordPolicy].
func NewWithPolicy(p PasswordPolicy) (string, error) {
	res := ""

	policy := []struct {
		count   int
		charset string
	}{
		{p.MinLowercase, lower},
		{p.MinUppercase, upper},
		{p.MinDigits, digits},
		{p.MinSymbols, symbols},
	}

	for _, p := range policy {
		s, err := generateRandomString(p.count, p.charset)
		if err != nil {
			return "", err
		}

		res += s
	}

	if missing := p.MinLength - len(res); missing > 0 {
		extra, err := generateRandomString(missing, defaultAlphabet)
		if err != nil {
			return "", err
		}

		res += extra
	}

	bs := []byte(res)
	if err := shuffle(bs); err != nil {
		return "", err
	}

	return string(bs), nil
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

// shuffle shuffles the given slice using the Fisher–Yates shuffle algorithm
// https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
func shuffle(bs []byte) error {
	for i := range bs {
		maxInt := big.NewInt(int64(i + 1))

		n, err := rand.Int(rand.Reader, maxInt)
		if err != nil {
			return err
		}

		j := n.Int64()
		bs[i], bs[j] = bs[j], bs[i]
	}

	return nil
}
