package vaulterrors

import "errors"

var (
	ErrVaultFileExists = errors.New("vault file already exists")

	ErrVaultFileNotFound = errors.New("vault file not found")

	ErrWrongPassword = errors.New("incorrect vault password")
)
