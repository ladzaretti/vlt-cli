package vaulterrors

import "errors"

var (
	ErrVaultFileExists = errors.New("vault file already exists")

	ErrVaultFileNotFound = errors.New("vault file does not exist")

	ErrWrongPassword = errors.New("incorrect vault password")

	ErrNonInteractiveUnsupported = errors.New("non-interactive input not supported")

	ErrEmptyName = errors.New("name cannot be empty")

	ErrEmptySecret = errors.New("secret cannot be empty")

	ErrMissingLabels = errors.New("missing required labels")

	ErrSearchNoMatch = errors.New("no match found")

	ErrAmbiguousSecretMatch = errors.New("ambiguous secret match: multiple secrets match the search criteria")
)
