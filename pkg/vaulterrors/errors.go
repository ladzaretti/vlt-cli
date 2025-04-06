package vaulterrors

import "errors"

var (
	// ErrVaultFileExists is returned when a vault file already exists.
	ErrVaultFileExists = errors.New("vault file already exists")

	// ErrVaultFileNotFound is returned when a vault file is not found.
	ErrVaultFileNotFound = errors.New("vault file not found")
)
