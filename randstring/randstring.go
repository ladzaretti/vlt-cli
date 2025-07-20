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

var DefaultPasswordPolicy = PasswordPolicy{
	MinLowercase: 2,
	MinUppercase: 2,
	MinNumeric:   2,
	MinSpecial:   2,
	MinLength:    12,
}

const (
	lower           = "abcdefghijklmnopqrstuvwxyz"
	upper           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	special         = "~`!@#$%^&*()_-+={[}]|\\:;\"'<,>.?/"
	numeric         = "0123456789"
	defaultAlphabet = numeric + upper + lower + special
)

type PasswordPolicy struct {
	MinLowercase int // MinLowercase is the minimum number of lowercase letters required.
	MinUppercase int // MinUppercase is the minimum number of uppercase letters required.
	MinNumeric   int // MinDigits is the minimum number of numeric digits required.
	MinSpecial   int // MinSymbols is the minimum number of special symbols required.
	MinLength    int // MinLength is the minimum total length of the password.
}

// New returns a securely generated random string of the given length.
func New(n int) ([]byte, error) {
	return generateRandomString(n, defaultAlphabet)
}

// NewWithAlphabet returns a securely generated random string using the provided alphabet.
func NewWithAlphabet(n int, alphabet string) ([]byte, error) {
	return generateRandomString(n, alphabet)
}

// NewWithPolicy generates a random password that satisfies the given [PasswordPolicy].
func NewWithPolicy(p PasswordPolicy) ([]byte, error) {
	res := make([]byte, 0, 2*p.MinLength)

	policy := []struct {
		count   int
		charset string
	}{
		{p.MinLowercase, lower},
		{p.MinUppercase, upper},
		{p.MinNumeric, numeric},
		{p.MinSpecial, special},
	}

	for _, p := range policy {
		if p.count <= 0 {
			continue
		}

		s, err := generateRandomString(p.count, p.charset)
		if err != nil {
			return nil, err
		}

		res = append(res, s...)
	}

	if missing := p.MinLength - len(res); missing > 0 {
		extra, err := generateRandomString(missing, defaultAlphabet)
		if err != nil {
			return nil, err
		}

		res = append(res, extra...)
	}

	if err := shuffle(res); err != nil {
		return nil, err
	}

	return res, nil
}

// generateRandomString returns a cryptographically secure random string using the given alphabet.
// It will return an error if the system's secure random
// number generator fails to function correctly.
func generateRandomString(n int, alphabet string) ([]byte, error) {
	if n <= 0 {
		return nil, ErrInvalidLength
	}

	if len(alphabet) == 0 {
		return nil, ErrEmptyAlphabet
	}

	ret := make([]byte, n)
	for i := range n {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return nil, err
		}

		ret[i] = alphabet[num.Int64()]
	}

	return ret, nil
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
